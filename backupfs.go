package backupfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
)

var (
	// assert interfaces implemented
	_ afero.Fs        = (*BackupFs)(nil)
	_ afero.Symlinker = (*BackupFs)(nil)

	// ErrRollbackFailed is returned when the rollback fails due to e.g. network problems.
	// when this error is returned it might make sense to retry the rollback
	ErrRollbackFailed = errors.New("rollback failed")

	// ErrNoSymlink is returned when we ar enot able to backup symlinks due to any of the base filesystem or
	// the target backup filesystem not supporting symlinks.
	ErrNoSymlink = afero.ErrNoSymlink
	// ErrBaseFsNoSymlink is returned in case that the base filesystem does not support symlinks
	ErrBaseFsNoSymlink = fmt.Errorf("base filesystem: %w", ErrNoSymlink)

	// ErrBackupFsNoSymlink is returned in case that the backup target filesystem does not support symlinks
	ErrBackupFsNoSymlink = fmt.Errorf("backup filesystem: %w", ErrNoSymlink)
)

// File is implemented by the imported directory.
type File = afero.File

// New creates a new layered backup file system that backups files from fs to backup in case that an
// existing file in fs is about to be overwritten or removed.
func NewBackupFs(base, backup afero.Fs) *BackupFs {
	return &BackupFs{
		base:   base,
		backup: backup,

		// this map is needed in order to keep track of non existing files
		// consecutive changes might lead to files being backed up
		// that were never there before
		// but could have been written by us in the mean time.
		// without this structure we would never know whether there was actually
		// no previous file to be backed up.
		baseInfos: make(map[string]os.FileInfo),
	}
}

// BackupFs is a file system abstraction that takes two underlying filesystems.
// One filesystem that is is being used to read and write files and a second filesystem
// which is used as backup target in case that a file of the base filesystem is about to be
// modified.
type BackupFs struct {
	// base filesystem which may be overwritten
	base afero.Fs
	// any initially overwritten file will be backed up to this filesystem
	backup afero.Fs

	// keeps track of base file system initial file state infos
	// os.FileInfo may be nil in case that the file never existed on the base
	// file system.
	// it is not nil in case that the file existed on the base file system
	baseInfos map[string]os.FileInfo
	mu        sync.Mutex
}

// GetBaseFs returns the fs layer that is being written to
func (fs *BackupFs) GetBaseFs() afero.Fs {
	return fs.base
}

// GetBackupFs returns the fs layer that is used to store the backups
func (fs *BackupFs) GetBackupFs() afero.Fs {
	return fs.backup
}

// The name of this FileSystem
func (fs *BackupFs) Name() string {
	return "BackupFs"
}

// Rollback tries to rollback the backup back to the
// base system removing any new files for the base
// system and restoring any old files from the backup
// Best effort, any errors due to filesystem
// modification on the backup site are skipped
// This is a heavy weight operation which blocks the file system
// until the rollback is done.
func (fs *BackupFs) Rollback() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// these file sneed to be removed in a certain order, so we keep track of them
	// from most nested to least nested files
	removeBaseFiles := make([]string, 0, 1)

	// these files also need to be restored in a certain order
	// from least nested to most nested
	restoreDirPaths := make([]string, 0, 4)
	restoreFilePaths := make([]string, 0, 4)
	restoreSymlinkPaths := make([]string, 0, 4)

	for path, info := range fs.baseInfos {
		if info == nil {
			// file did not exist in the base filesystem at the point of
			// filesystem modification.
			exists, err := internal.LExists(fs.base, path)
			if err == nil && exists {
				// we will need to delete this file
				removeBaseFiles = append(removeBaseFiles, path)
			}

			// case where file must be removed in base file system
			// finished
			continue
		}

		mode := info.Mode()
		switch {
		case mode.IsDir():
			restoreDirPaths = append(restoreDirPaths, path)
		case mode.IsRegular():
			restoreFilePaths = append(restoreFilePaths, path)
		case mode&os.ModeSymlink != 0:
			restoreSymlinkPaths = append(restoreSymlinkPaths, path)
		default:
			log.Printf("unknown file type: %s\n", path)
		}
	}

	// remove files from most nested to least nested
	sort.Sort(internal.ByMostFilePathSeparators(removeBaseFiles))
	for _, remPath := range removeBaseFiles {
		// remove all files that were not there before the backup.
		// ignore error, as this is a best effort restoration.
		_ = fs.base.Remove(remPath)
	}

	// in order to iterate over parent directories before child directories
	sort.Sort(internal.ByLeastFilePathSeparators(restoreDirPaths))

	for _, dirPath := range restoreDirPaths {
		// backup -> base filesystem
		err := internal.CopyDir(fs.base, dirPath, fs.baseInfos[dirPath])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRollbackFailed, err)
		}
	}

	// in this case it does not matter whether we sort the file paths or not
	// we prefer to sort them in order to see potential errors better
	sort.Strings(restoreFilePaths)

	for _, filePath := range restoreFilePaths {
		err := internal.RestoreFile(filePath, fs.baseInfos[filePath], fs.base, fs.backup)
		if err != nil {
			// in this case it might make sense to retry the rollback
			return fmt.Errorf("%w: %v", ErrRollbackFailed, err)
		}
	}

	// in this case it does not matter whether we sort the symlink paths or not
	// we prefer to sort them in order to see potential errors better
	sort.Strings(restoreSymlinkPaths)
	restoredSymlinks := make([]string, 0, 4)
	for _, symlinkPath := range restoreSymlinkPaths {
		err := internal.RestoreSymlink(
			symlinkPath,
			fs.baseInfos[symlinkPath],
			fs.base,
			fs.backup,
			ErrBaseFsNoSymlink,
			ErrBackupFsNoSymlink,
		)
		if errors.Is(err, ErrNoSymlink) {
			// at least one of the filesystems does not support symlinks
			break
		} else if err != nil {
			// in this case it might make sense to retry the rollback
			return fmt.Errorf("%w: %v", ErrRollbackFailed, err)
		}
		restoredSymlinks = append(restoredSymlinks, symlinkPath)
	}

	// TODO: make this optional?: whether to delete the backup upon rollback

	// at this point we were able to restore all of the files
	// now we need to delete our backup

	for _, symlinkPath := range restoredSymlinks {
		// best effort deletion of backup files
		// so we ignore the error
		_ = fs.backup.Remove(symlinkPath)
	}

	// delete all files first
	for _, filePath := range restoreFilePaths {
		// best effort deletion of backup files
		// so we ignore the error
		_ = fs.backup.Remove(filePath)
	}

	// we want to delete all of the backed up folders from
	// the most nested child directories to the least nested parent directories.
	sort.Sort(internal.ByMostFilePathSeparators(restoreDirPaths))

	// delete all files first
	for _, dirPath := range restoreDirPaths {
		// best effort deletion of backup files
		// so we ignore the error
		// we only delete directories that we did create.
		// any user created content in directories is not touched
		_ = fs.backup.Remove(dirPath)
	}

	// at this point we have successfully restored our backup and
	// removed all of the backup files and directories

	// now we can reset the internal data structure for book keeping of filesystem modifications
	fs.baseInfos = make(map[string]os.FileInfo)
	return nil
}

type fInfo struct {
	Name    string `json:"name"`
	Mode    uint32 `json:"mode"`
	ModTime int64  `json:"mod_time"`
	Uid     int    `json:"uid"`
	Gid     int    `json:"gid"`
}

func (fs *BackupFs) MarshalJSON() ([]byte, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fiMap := make(map[string]*fInfo, len(fs.baseInfos))

	for path, fi := range fs.baseInfos {
		if fi == nil {
			fiMap[path] = nil
			continue
		}

		fiMap[path] = &fInfo{
			Name:    filepath.ToSlash(path),
			Mode:    uint32(fi.Mode()),
			ModTime: fi.ModTime().UnixNano(),
			Uid:     internal.Uid(fi),
			Gid:     internal.Gid(fi),
		}
	}

	return json.Marshal(fiMap)
}

func (fs *BackupFs) UnmarshalJSON(data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fiMap := make(map[string]*fInfo)

	err := json.Unmarshal(data, &fiMap)
	if err != nil {
		return err
	}

	result := make(map[string]os.FileInfo, len(fiMap))

	for path, fi := range fiMap {
		path = filepath.FromSlash(path)
		if fi == nil {
			result[path] = nil
			continue
		}

		var memFile *mem.FileData
		mode := os.FileMode(fi.Mode)
		if mode.IsDir() {
			memFile = mem.CreateDir(fi.Name)
		} else {
			memFile = mem.CreateFile(fi.Name)
		}

		mem.SetMode(memFile, mode)
		mem.SetModTime(memFile, time.Unix(fi.ModTime/1000, fi.ModTime%1000))

		if fi.Gid >= 0 {
			mem.SetGID(memFile, fi.Gid)
		}

		if fi.Uid >= 0 {
			mem.SetUID(memFile, fi.Uid)
		}

		result[path] = mem.GetFileInfo(memFile)
	}

	fs.baseInfos = result
	return nil
}

// returns the cleaned path
func (fs *BackupFs) realPath(name string) (path string, err error) {
	if runtime.GOOS == "windows" && filepath.IsAbs(name) {
		// On Windows a common mistake would be to provide an absolute OS path
		// We could strip out the base part, but that would not be very portable.

		return name, os.ErrNotExist
	}

	return filepath.Clean(name), nil
}

// keeps track of files in the base filesystem.
// Files are saved only once, any consecutive update is ignored.
func (fs *BackupFs) setBaseInfoIfNotFound(path string, info os.FileInfo) {
	_, found := fs.baseInfos[path]
	if !found {
		fs.baseInfos[path] = info
	}
}

// alreadyFoundBaseInfo returns true when we already visited this path.
// This is a helper function in order to NOT call the Stat method of the baseFs
// an unnecessary amount of times for filepath sub directories when we can just lookup
// the information in out internal filepath map
func (fs *BackupFs) alreadyFoundBaseInfo(path string) bool {
	_, found := fs.baseInfos[path]
	return found
}

// Stat returns a FileInfo describing the named file, or an error, if any happens.
// Stat only looks at the base filesystem and returns the stat of the files at the specified path
func (fs *BackupFs) Stat(name string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.stat(name)
}

func (fs *BackupFs) stat(name string) (os.FileInfo, error) {
	name, err := fs.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	// we want to check all parent directories before we check the actual file.
	// in order to keep track of their state as well.
	// /root -> /root/sub/ -> /root/sub/sub1
	// iterate parent directories and keep track of their initial state.
	internal.IterateDirTree(filepath.Dir(name), func(subdirPath string) error {
		if fs.alreadyFoundBaseInfo(subdirPath) {
			return nil
		}

		// only in case that we have not yet visited one of the subdirs already,
		// only then fetch the file information from the underlying baseFs
		// we do want to ignore errors as this is only for keeping track of subdirectories
		_, _ = fs.trackedStat(subdirPath)
		return nil
	})

	return fs.trackedStat(name)
}

// trackedStat is the tracked variant of Stat that is called on the underlying base Fs
func (fs *BackupFs) trackedStat(name string) (os.FileInfo, error) {
	fi, err := fs.base.Stat(name)

	// keep track of initial
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if oerr.Err == os.ErrNotExist || oerr.Err == syscall.ENOENT || oerr.Err == syscall.ENOTDIR {

				fs.setBaseInfoIfNotFound(name, nil)
				return nil, err
			}
		}
		if err == syscall.ENOENT {
			fs.setBaseInfoIfNotFound(name, nil)
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	fs.setBaseInfoIfNotFound(name, fi)
	return fi, nil
}

// backupRequired checks whether a file that is about to be changed needs to be backed up.
// files that do not exist in the backupFs need to be backed up.
// files that do exist in the backupFs either as files or in the baseInfos map as non-existing files
// do not  need to be backed up (again)
func (fs *BackupFs) backupRequired(path string) (info os.FileInfo, required bool, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	info, found := fs.baseInfos[path]
	if !found {
		// fill fs.baseInfos
		// of symlink, file & directory as well as their parent directories.
		info, _, err = fs.lstatIfPossible(path)
		if err != nil && os.IsNotExist(err) {
			// not found, no backup needed
			return nil, false, nil
		} else if err != nil {
			return nil, false, err
		}
		// err == nil
	}

	// at this point info is either set by baseInfos or by fs.tat
	if info == nil {
		//actually no file expected at that location
		return nil, false, nil
	}

	// file found at base fs location

	// did we already backup that file?
	foundBackup, err := internal.LExists(fs.backup, path)
	if err != nil {
		return nil, false, err
	}

	if foundBackup {
		// no need to backup, as we already backed up the file
		return nil, false, nil
	}

	// backup is needed
	return info, true, nil
}

func (fs *BackupFs) tryBackup(name string) (err error) {

	info, needsBackup, err := fs.backupRequired(name)
	if err != nil {
		return err
	}
	if !needsBackup {
		return nil
	}

	dirPath := name
	if !info.IsDir() {
		// is file, get dir
		dirPath = filepath.Dir(dirPath)
	}

	err = internal.IterateDirTree(dirPath, func(subDirPath string) error {
		fi, required, err := fs.backupRequired(subDirPath)
		if err != nil {
			return err
		}

		if !required {
			return nil
		}

		return internal.CopyDir(fs.backup, subDirPath, fi)
	})
	if err != nil {
		return err
	}

	fileMode := info.Mode()
	switch {
	case fileMode.IsDir():
		// file was actually a directory,
		// we did already backup all of the directory tree
		return nil

	case fileMode.IsRegular():
		// name was a path to a file
		// create the file
		sf, err := fs.base.Open(name)
		if err != nil {
			return err
		}
		defer sf.Close()
		return internal.CopyFile(fs.backup, name, info, sf)

	case fileMode&os.ModeSymlink != 0:
		// symlink
		return internal.CopySymlink(
			fs.base,
			fs.backup,
			name,
			info,
			ErrBackupFsNoSymlink,
			ErrBackupFsNoSymlink,
		)

	default:
		// unsupported file for backing up
		return nil
	}
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (fs *BackupFs) Create(name string) (File, error) {
	name, err := fs.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}
	// create or truncate file
	file, err := fs.base.Create(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("create failed: %w", err)}
	}
	return file, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (fs *BackupFs) Mkdir(name string, perm os.FileMode) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fs.base.Mkdir(name, perm)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("mkdir failed: %w", err)}
	}
	return nil
}

// MkdirAll creates a directory path and all
// parents that does not exist yet.
func (fs *BackupFs) MkdirAll(name string, perm os.FileMode) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fs.base.MkdirAll(name, perm)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("mkdir_all failed: %w", err)}
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (fs *BackupFs) Open(name string) (File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (fs *BackupFs) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	name, err := fs.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	if flag == os.O_RDONLY {
		// in read only mode the perm is not used.
		return fs.base.OpenFile(name, os.O_RDONLY, 0)
	}

	// not read only opening -> backup
	err = fs.tryBackup(name)
	if err != nil {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	file, err := fs.base.OpenFile(name, flag, perm)
	if err != nil {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: fmt.Errorf("openfile failed: %w", err)}
	}
	return file, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (fs *BackupFs) Remove(name string) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fs.base.Remove(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("remove failed: %w", err)}
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
// not supported
func (fs *BackupFs) RemoveAll(name string) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	fi, err := fs.Stat(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// if it's a file, directly remove it
	if !fi.IsDir() {
		err = fs.Remove(name)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("remove failed: %w", err)}
		}
		return nil
	}

	directoryPaths := make([]string, 0, 1)

	err = afero.Walk(fs.base, name, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// initially we want to delete all files before we delete all of the directories
			// but we also want to keep track of all found directories in order not to walk the
			// dir tree again.
			directoryPaths = append(directoryPaths, path)
			return nil
		}

		return fs.Remove(path)
	})

	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("walkdir or remove failed: %w", err)}
	}

	// after deleting all of the files
	//now we want to sort all of the file paths from the most
	//nested file to the least nested file (count file path separators)
	sort.Sort(internal.ByMostFilePathSeparators(directoryPaths))

	for _, path := range directoryPaths {
		err = fs.Remove(path)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: err}
		}
	}

	return nil
}

// Rename renames a file.
func (fs *BackupFs) Rename(oldname, newname string) error {
	newname, err := fs.realPath(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: fmt.Errorf("failed to clean newname: %w", err)}
	}

	oldname, err = fs.realPath(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("failed to clean oldname: %w", err)}
	}

	// make target file known
	err = fs.tryBackup(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: fmt.Errorf("failed to backup newname: %w", err)}
	}

	// there either was no previous file to be backed up
	// but now we know that there was no file or there
	// was a target file that has to be backed up which was then backed up

	err = fs.tryBackup(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("failed to backup oldname: %w", err)}
	}

	err = fs.base.Rename(oldname, newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("renaming failed: %w", err)}
	}
	return nil
}

// Chmod changes the mode of the named file to mode.
func (fs *BackupFs) Chmod(name string, mode os.FileMode) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("failed to get clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fs.base.Chmod(name, mode)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("chmod failed: %w", err)}
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (fs *BackupFs) Chown(name string, uid, gid int) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	// TODO: do we want to ignore errors from Windows that this function is not supported by the OS?
	err = fs.base.Chown(name, uid, gid)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("chown failed: %w", err)}
	}
	return nil
}

//Chtimes changes the access and modification times of the named file
func (fs *BackupFs) Chtimes(name string, atime, mtime time.Time) error {
	name, err := fs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}
	err = fs.base.Chtimes(name, atime, mtime)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("chtimes failed: %w", err)}
	}
	return nil
}

func (fs *BackupFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.lstatIfPossible(name)
}

// lstatIfPossible has the same logic as stat
func (fs *BackupFs) lstatIfPossible(name string) (os.FileInfo, bool, error) {
	name, err := fs.realPath(name)
	if err != nil {
		return nil, false, err
	}

	// we want to check all parent directories before we check the actual file.
	// in order to keep track of their state as well.
	// /root -> /root/sub/ -> /root/sub/sub1
	// iterate parent directories and keep track of their initial state.
	internal.IterateDirTree(filepath.Dir(name), func(subdirPath string) error {
		if fs.alreadyFoundBaseInfo(subdirPath) {
			return nil
		}

		// only in the case that we do not know the subdirectory already
		// we do want to track the initial state of the sub directory.
		// if it does not exist, it should not exist
		_, _ = fs.trackedStat(subdirPath)
		return nil
	})

	fi, lstatCalled, err := fs.trackedLstat(name)
	if err != nil {
		return nil, lstatCalled, err
	}

	return fi, lstatCalled, nil
}

// trackedLstat has the same logic as trackedStat but it uses Lstat instead, in case that is possible.
func (fs *BackupFs) trackedLstat(name string) (os.FileInfo, bool, error) {
	var (
		fi  os.FileInfo
		err error
		// only set to true when lstat is called
		lstatCalled = false
	)

	baseLstater, ok := internal.LstaterIfPossible(fs.base)
	if ok {
		fi, lstatCalled, err = baseLstater.LstatIfPossible(name)
	} else {
		fi, err = fs.base.Stat(name)

	}

	// keep track of initial
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if oerr.Err == os.ErrNotExist || oerr.Err == syscall.ENOENT || oerr.Err == syscall.ENOTDIR {
				// file or symlink does not exist
				fs.setBaseInfoIfNotFound(name, nil)
				return nil, lstatCalled, err
			}
		}
		if err == syscall.ENOENT {
			// file or symlink does not exist
			fs.setBaseInfoIfNotFound(name, nil)
			return nil, lstatCalled, err
		}
	}

	if err != nil {
		return nil, lstatCalled, err
	}

	fs.setBaseInfoIfNotFound(name, fi)
	return fi, lstatCalled, nil
}

//SymlinkIfPossible changes the access and modification times of the named file
func (fs *BackupFs) SymlinkIfPossible(oldname, newname string) error {
	oldname, err := fs.realPath(oldname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to clean oldname: %w", err)}
	}

	newname, err = fs.realPath(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to clean newname: %w", err)}
	}

	// we only want to backup the newname,
	// as seemingly the new name is the target symlink location
	// the old file path should not have been modified

	// in case we fail to backup the symlink, we return an error
	err = fs.tryBackup(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to backup newname: %w", err)}
	}

	if linker, ok := fs.base.(afero.Linker); ok {
		err = linker.SymlinkIfPossible(oldname, newname)
		if err != nil {
			return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("symlink failed: %w", err)}
		}
		return nil
	}
	return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: afero.ErrNoSymlink}
}

func (fs *BackupFs) ReadlinkIfPossible(name string) (string, error) {
	name, err := fs.realPath(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	if reader, ok := fs.base.(afero.LinkReader); ok {
		path, err := reader.ReadlinkIfPossible(name)
		if err != nil {
			return "", &os.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("readlink failed: %w", err)}
		}
		return path, nil
	}
	return "", &os.PathError{Op: "readlink", Path: name, Err: afero.ErrNoReadlink}
}
