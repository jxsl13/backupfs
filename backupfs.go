package backupfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/jxsl13/backupfs/fsutils"
	"github.com/jxsl13/backupfs/interfaces"
	"github.com/jxsl13/backupfs/internal"
)

var (
	// assert interfaces implemented
	_ interfaces.Fs = (*BackupFs)(nil)

	// ErrRollbackFailed is returned when the rollback fails due to e.g. network problems.
	// when this error is returned it might make sense to retry the rollback
	ErrRollbackFailed = errors.New("rollback failed")
)

// NewBackupFs creates a new layered backup file system that backups files from fs to backup in case that an
// existing file in fs is about to be overwritten or removed.
func NewBackupFs(base, backup interfaces.Fs) *BackupFs {
	return &BackupFs{
		base:   base,
		backup: backup,

		// this map is needed in order to keep track of non existing files
		// consecutive changes might lead to files being backed up
		// that were never there before
		// but could have been written by us in the mean time.
		// without this structure we would never know whether there was actually
		// no previous file to be backed up.
		baseInfos: make(map[string]fs.FileInfo),
	}
}

// NewBackupFsWithVolume creates a new layered backup file system that backups files from fs to backup in case that an
// existing file in fs is about to be overwritten or removed.
// Contrary to the normal backupfs this variant allows to use absolute windows paths (C:\A\B\C instead of \A\B\C)
func NewBackupFsWithVolume(base, backup interfaces.Fs) *BackupFs {
	return &BackupFs{
		base:               base,
		backup:             backup,
		windowsVolumePaths: true,

		// this map is needed in order to keep track of non existing files
		// consecutive changes might lead to files being backed up
		// that were never there before
		// but could have been written by us in the mean time.
		// without this structure we would never know whether there was actually
		// no previous file to be backed up.
		baseInfos: make(map[string]fs.FileInfo),
	}
}

// BackupFs is a file system abstraction that takes two underlying filesystems.
// One filesystem that is is being used to read and write files and a second filesystem
// which is used as backup target in case that a file of the base filesystem is about to be
// modified.
type BackupFs struct {
	// base filesystem which may be overwritten
	base interfaces.Fs
	// any initially overwritten file will be backed up to this filesystem
	backup interfaces.Fs

	// keeps track of base file system initial file state infos
	// fs.FileInfo may be nil in case that the file never existed on the base
	// file system.
	// it is not nil in case that the file existed on the base file system
	baseInfos map[string]fs.FileInfo
	mu        sync.Mutex

	// windowsVolumePaths can be set to true in order to allow fully
	// qualified windows paths (including volume names C:\A\B\C instead of \A\B\C)
	windowsVolumePaths bool
}

// GetBaseFs returns the fs layer that is being written to
func (bfs *BackupFs) GetBaseFs() interfaces.Fs {
	return bfs.base
}

// GetBackupFs returns the fs layer that is used to store the backups
func (bfs *BackupFs) GetBackupFs() interfaces.Fs {
	return bfs.backup
}

// The name of this FileSystem
func (bfs *BackupFs) Name() string {
	return "BackupFs"
}

// Rollback tries to rollback the backup back to the
// base system removing any new files for the base
// system and restoring any old files from the backup
// Best effort, any errors due to filesystem
// modification on the backup site are skipped
// This is a heavy weight operation which blocks the file system
// until the rollback is done.
func (bfs *BackupFs) Rollback() error {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	// these file sneed to be removed in a certain order, so we keep track of them
	// from most nested to least nested files
	removeBaseFiles := make([]string, 0, 1)

	// these files also need to be restored in a certain order
	// from least nested to most nested
	restoreDirPaths := make([]string, 0, 4)
	restoreFilePaths := make([]string, 0, 4)
	restoreSymlinkPaths := make([]string, 0, 4)

	for path, info := range bfs.baseInfos {
		if info == nil {
			// file did not exist in the base filesystem at the point of
			// filesystem modification.
			exists, err := fsutils.LExists(bfs.base, path)
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
		_ = bfs.base.Remove(remPath)
	}

	// in order to iterate over parent directories before child directories
	sort.Sort(internal.ByLeastFilePathSeparators(restoreDirPaths))

	for _, dirPath := range restoreDirPaths {
		// backup -> base filesystem
		err := internal.CopyDir(bfs.base, dirPath, bfs.baseInfos[dirPath])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRollbackFailed, err)
		}
	}

	// in this case it does not matter whether we sort the file paths or not
	// we prefer to sort them in order to see potential errors better
	sort.Strings(restoreFilePaths)

	for _, filePath := range restoreFilePaths {
		err := internal.RestoreFile(filePath, bfs.baseInfos[filePath], bfs.base, bfs.backup)
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
			bfs.baseInfos[symlinkPath],
			bfs.base,
			bfs.backup,
		)
		if err != nil {
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
		_ = bfs.backup.Remove(symlinkPath)
	}

	// delete all files first
	for _, filePath := range restoreFilePaths {
		// best effort deletion of backup files
		// so we ignore the error
		_ = bfs.backup.Remove(filePath)
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
		_ = bfs.backup.Remove(dirPath)
	}

	// at this point we have successfully restored our backup and
	// removed all of the backup files and directories

	// now we can reset the internal data structure for book keeping of filesystem modifications
	bfs.baseInfos = make(map[string]fs.FileInfo)
	return nil
}

func (bfs *BackupFs) Map() map[string]fs.FileInfo {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	m := make(map[string]fs.FileInfo, len(bfs.baseInfos))
	for path, info := range bfs.baseInfos {
		m[path] = toFInfo(path, info)
	}
	return m
}

func toFInfo(path string, fi fs.FileInfo) *fInfo {
	return &fInfo{
		FileName:    filepath.ToSlash(path),
		FileMode:    uint32(fi.Mode()),
		FileModTime: fi.ModTime().UnixNano(),
		FileSize:    fi.Size(),
	}
}

type fInfo struct {
	FileName    string `json:"name"`
	FileMode    uint32 `json:"mode"`
	FileModTime int64  `json:"mod_time"`
	FileSize    int64  `json:"size"`
}

func (fi *fInfo) Name() string {
	return path.Base(fi.FileName)
}
func (fi *fInfo) Size() int64 {
	return fi.FileSize
}
func (fi *fInfo) Mode() os.FileMode {
	return os.FileMode(fi.FileMode)
}
func (fi *fInfo) ModTime() time.Time {
	return time.Unix(fi.FileModTime/1000000000, fi.FileModTime%1000000000)
}
func (fi *fInfo) IsDir() bool {
	return fi.Mode().IsDir()
}
func (fi *fInfo) Sys() interface{} {
	return nil
}

func (bfs *BackupFs) MarshalJSON() ([]byte, error) {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	fiMap := make(map[string]*fInfo, len(bfs.baseInfos))

	for path, fi := range bfs.baseInfos {
		if fi == nil {
			fiMap[path] = nil
			continue
		}

		fiMap[path] = toFInfo(path, fi)
	}

	return json.Marshal(fiMap)
}

func (bfs *BackupFs) UnmarshalJSON(data []byte) error {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	fiMap := make(map[string]*fInfo)

	err := json.Unmarshal(data, &fiMap)
	if err != nil {
		return err
	}

	bfs.baseInfos = make(map[string]fs.FileInfo, len(fiMap))
	for k, v := range fiMap {
		bfs.baseInfos[k] = v
	}

	return nil
}

// returns the cleaned path
func (bfs *BackupFs) realPath(name string) (path string, err error) {
	// check path for being an absolute windows path
	if !bfs.windowsVolumePaths && runtime.GOOS == "windows" && filepath.IsAbs(name) {
		// On Windows a common mistake would be to provide an absolute OS path
		// We could strip out the base part, but that would not be very portable.

		return name, os.ErrNotExist
	}

	return filepath.Clean(name), nil
}

// keeps track of files in the base filesystem.
// Files are saved only once, any consecutive update is ignored.
func (bfs *BackupFs) setBaseInfoIfNotFound(path string, info fs.FileInfo) {
	_, found := bfs.baseInfos[path]
	if !found {
		bfs.baseInfos[path] = info
	}
}

// alreadyFoundBaseInfo returns true when we already visited this path.
// This is a helper function in order to NOT call the Stat method of the baseFs
// an unnecessary amount of times for filepath sub directories when we can just lookup
// the information in out internal filepath map
func (bfs *BackupFs) alreadyFoundBaseInfo(path string) bool {
	_, found := bfs.baseInfos[path]
	return found
}

// Stat returns a FileInfo describing the named file, or an error, if any happens.
// Stat only looks at the base filesystem and returns the stat of the files at the specified path
func (bfs *BackupFs) Stat(name string) (fs.FileInfo, error) {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	return bfs.stat(name)
}

func (bfs *BackupFs) stat(name string) (fs.FileInfo, error) {
	name, err := bfs.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	// we want to check all parent directories before we check the actual file.
	// in order to keep track of their state as well.
	// /root -> /root/sub/ -> /root/sub/sub1
	// iterate parent directories and keep track of their initial state.
	fsutils.IterateDirTree(filepath.Dir(name), func(subdirPath string) error {
		if bfs.alreadyFoundBaseInfo(subdirPath) {
			return nil
		}

		// only in case that we have not yet visited one of the subdirs already,
		// only then fetch the file information from the underlying baseFs
		// we do want to ignore errors as this is only for keeping track of subdirectories
		_, _ = bfs.trackedStat(subdirPath)
		return nil
	})

	return bfs.trackedStat(name)
}

// trackedStat is the tracked variant of Stat that is called on the underlying base Fs
func (bfs *BackupFs) trackedStat(name string) (fs.FileInfo, error) {
	fi, err := bfs.base.Stat(name)

	// keep track of initial
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if errors.Is(oerr.Err, fs.ErrNotExist) || errors.Is(oerr.Err, syscall.ENOENT) || errors.Is(oerr.Err, syscall.ENOTDIR) {

				bfs.setBaseInfoIfNotFound(name, nil)
				return nil, err
			}
		}
		if errors.Is(err, syscall.ENOENT) {
			bfs.setBaseInfoIfNotFound(name, nil)
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	bfs.setBaseInfoIfNotFound(name, fi)
	return fi, nil
}

// backupRequired checks whether a file that is about to be changed needs to be backed up.
// files that do not exist in the backupFs need to be backed up.
// files that do exist in the backupFs either as files or in the baseInfos map as non-existing files
// do not  need to be backed up (again)
func (bfs *BackupFs) backupRequired(path string) (info fs.FileInfo, required bool, err error) {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	info, found := bfs.baseInfos[path]
	if !found {
		// fill fs.baseInfos
		// of symlink, file & directory as well as their parent directories.
		info, err = bfs.lstat(path)
		if err != nil && errors.Is(err, fs.ErrNotExist) {
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
	foundBackup, err := fsutils.LExists(bfs.backup, path)
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

func (bfs *BackupFs) ForceBackup(name string) (err error) {
	name, err = bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "force_backup", Err: fmt.Errorf("failed to clean path: %w", err), Path: name}
	}

	err = bfs.tryRemoveBackup(name)
	if err != nil {
		return &os.PathError{Op: "force_backup", Err: fmt.Errorf("failed to remove backup: %w", err), Path: name}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "force_backup", Err: fmt.Errorf("backup failed: %w", err), Path: name}
	}

	return nil
}

func (bfs *BackupFs) tryRemoveBackup(name string) (err error) {
	_, needsBackup, err := bfs.backupRequired(name)
	if err != nil {
		return err
	}
	// there is no backup
	if needsBackup {
		return nil
	}

	fi, err := bfs.backup.Lstat(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// file not found
	if fi == nil {
		// nothing to remove, except internal state if it exists
		bfs.mu.Lock()
		defer bfs.mu.Unlock()

		delete(bfs.baseInfos, name)
		return nil
	}

	if !fi.IsDir() {
		defer func() {
			if err == nil {
				// only delete from internal state
				// when file has been deleted
				// this allows to retry the deletion attempt
				bfs.mu.Lock()
				delete(bfs.baseInfos, name)
				bfs.mu.Unlock()
			}
		}()
		// remove file/symlink
		return bfs.backup.Remove(name)
	}

	dirs := make([]string, 0)

	err = fsutils.Walk(bfs.backup, name, func(path string, info fs.FileInfo, err error) (e error) {
		// and then check for error
		if err != nil {
			return err
		}

		defer func() {
			if e == nil {
				// delete dirs and files from internal map
				// but only after re have removed the file successfully
				bfs.mu.Lock()
				delete(bfs.baseInfos, path)
				bfs.mu.Unlock()
			}
		}()

		if info.IsDir() {
			// keep track of dirs
			dirs = append(dirs, path)
			return nil
		}

		// delete files
		return bfs.backup.Remove(path)
	})
	if err != nil {
		return err
	}

	sort.Sort(internal.ByMostFilePathSeparators(dirs))

	for _, dir := range dirs {
		err = bfs.backup.RemoveAll(dir)
		if err != nil {
			return err
		}

		// delete directory from internal
		// state only after it has been actually deleted
		bfs.mu.Lock()
		delete(bfs.baseInfos, dir)
		bfs.mu.Unlock()
	}

	return nil
}

func (bfs *BackupFs) tryBackup(name string) (err error) {

	info, needsBackup, err := bfs.backupRequired(name)
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

	err = fsutils.IterateDirTree(dirPath, func(subDirPath string) error {
		fi, required, err := bfs.backupRequired(subDirPath)
		if err != nil {
			return err
		}

		if !required {
			return nil
		}

		return internal.CopyDir(bfs.backup, subDirPath, fi)
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
		sf, err := bfs.base.Open(name)
		if err != nil {
			return err
		}
		defer sf.Close()
		return internal.CopyFile(bfs.backup, name, info, sf)

	case fileMode&os.ModeSymlink != 0:
		// symlink
		return internal.CopySymlink(
			bfs.base,
			bfs.backup,
			name,
			info,
		)

	default:
		// unsupported file for backing up
		return nil
	}
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (bfs *BackupFs) Create(name string) (interfaces.File, error) {
	name, err := bfs.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}
	// create or truncate file
	file, err := bfs.base.Create(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("create failed: %w", err)}
	}
	return file, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (bfs *BackupFs) Mkdir(name string, perm os.FileMode) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = bfs.base.Mkdir(name, perm)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("mkdir failed: %w", err)}
	}
	return nil
}

// MkdirAll creates a directory path and all
// parents that does not exist yet.
func (bfs *BackupFs) MkdirAll(name string, perm os.FileMode) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = bfs.base.MkdirAll(name, perm)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("mkdir_all failed: %w", err)}
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (bfs *BackupFs) Open(name string) (interfaces.File, error) {
	return bfs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (bfs *BackupFs) OpenFile(name string, flag int, perm os.FileMode) (interfaces.File, error) {
	name, err := bfs.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	if flag == os.O_RDONLY {
		// in read only mode the perm is not used.
		return bfs.base.OpenFile(name, os.O_RDONLY, 0)
	}

	// not read only opening -> backup
	err = bfs.tryBackup(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	file, err := bfs.base.OpenFile(name, flag, perm)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("open failed: %w", err)}
	}
	return file, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (bfs *BackupFs) Remove(name string) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = bfs.base.Remove(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("remove failed: %w", err)}
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
// not supported
func (bfs *BackupFs) RemoveAll(name string) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	fi, err := bfs.Lstat(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// if it's a file or a symlink, directly remove it
	if !fi.IsDir() {
		err = bfs.Remove(name)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("remove failed: %w", err)}
		}
		return nil
	}

	directoryPaths := make([]string, 0, 1)

	err = fsutils.Walk(bfs.base, name, func(path string, info fs.FileInfo, err error) error {
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

		return bfs.Remove(path)
	})

	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("walkdir or remove failed: %w", err)}
	}

	// after deleting all of the files
	//now we want to sort all of the file paths from the most
	//nested file to the least nested file (count file path separators)
	sort.Sort(internal.ByMostFilePathSeparators(directoryPaths))

	for _, path := range directoryPaths {
		err = bfs.Remove(path)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: err}
		}
	}

	return nil
}

// Rename renames a file.
func (bfs *BackupFs) Rename(oldname, newname string) error {
	newname, err := bfs.realPath(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: fmt.Errorf("failed to clean newname: %w", err)}
	}

	oldname, err = bfs.realPath(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("failed to clean oldname: %w", err)}
	}

	// make target file known
	err = bfs.tryBackup(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: fmt.Errorf("failed to backup newname: %w", err)}
	}

	// there either was no previous file to be backed up
	// but now we know that there was no file or there
	// was a target file that has to be backed up which was then backed up

	err = bfs.tryBackup(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("failed to backup oldname: %w", err)}
	}

	err = bfs.base.Rename(oldname, newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("renaming failed: %w", err)}
	}
	return nil
}

// Chmod changes the mode of the named file to mode.
func (bfs *BackupFs) Chmod(name string, mode os.FileMode) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("failed to get clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = bfs.base.Chmod(name, mode)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("chmod failed: %w", err)}
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (bfs *BackupFs) Chown(name string, username, groupname string) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = bfs.base.Chown(name, username, groupname)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("chown failed: %w", err)}
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (bfs *BackupFs) Chtimes(name string, atime, mtime time.Time) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}
	err = bfs.base.Chtimes(name, atime, mtime)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("chtimes failed: %w", err)}
	}
	return nil
}

func (bfs *BackupFs) Lstat(name string) (fs.FileInfo, error) {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	return bfs.lstat(name)
}

// lstat has the same logic as stat
func (bfs *BackupFs) lstat(name string) (fs.FileInfo, error) {
	name, err := bfs.realPath(name)
	if err != nil {
		return nil, err
	}

	// we want to check all parent directories before we check the actual file.
	// in order to keep track of their state as well.
	// /root -> /root/sub/ -> /root/sub/sub1
	// iterate parent directories and keep track of their initial state.
	fsutils.IterateDirTree(filepath.Dir(name), func(subdirPath string) error {
		if bfs.alreadyFoundBaseInfo(subdirPath) {
			return nil
		}

		// only in the case that we do not know the subdirectory already
		// we do want to track the initial state of the sub directory.
		// if it does not exist, it should not exist
		_, _ = bfs.trackedStat(subdirPath)
		return nil
	})

	fi, err := bfs.trackedLstat(name)
	if err != nil {
		return nil, err
	}

	return fi, nil
}

// trackedLstat has the same logic as trackedStat but it uses Lstat instead, in case that is possible.
func (bfs *BackupFs) trackedLstat(name string) (fs.FileInfo, error) {

	fi, err := bfs.base.Lstat(name)

	// keep track of initial
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if errors.Is(oerr.Err, fs.ErrNotExist) || errors.Is(oerr.Err, syscall.ENOENT) || errors.Is(oerr.Err, syscall.ENOTDIR) {
				// file or symlink does not exist
				bfs.setBaseInfoIfNotFound(name, nil)
				return nil, err
			}
		}
		if errors.Is(err, syscall.ENOENT) {
			// file or symlink does not exist
			bfs.setBaseInfoIfNotFound(name, nil)
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	bfs.setBaseInfoIfNotFound(name, fi)
	return fi, nil
}

// Symlink changes the access and modification times of the named file
func (bfs *BackupFs) Symlink(oldname, newname string) error {
	oldname, err := bfs.realPath(oldname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to clean oldname: %w", err)}
	}

	newname, err = bfs.realPath(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to clean newname: %w", err)}
	}

	// we only want to backup the newname,
	// as seemingly the new name is the target symlink location
	// the old file path should not have been modified

	// in case we fail to backup the symlink, we return an error
	err = bfs.tryBackup(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to backup newname: %w", err)}
	}

	err = bfs.base.Symlink(oldname, newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("symlink failed: %w", err)}
	}
	return nil
}

func (bfs *BackupFs) Readlink(name string) (string, error) {
	name, err := bfs.realPath(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	path, err := bfs.base.Readlink(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("readlink failed: %w", err)}
	}

	return path, nil
}

// Lchown does not fallback to chown. It does return an error in case that lchown cannot be called.
func (bfs *BackupFs) Lchown(name string, username, groupname string) error {
	name, err := bfs.realPath(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	//TODO: check if the owner stays equal and then backup the file if the owner changes
	// at this point we do modify the owner -> require backup
	err = bfs.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	return bfs.base.Lchown(name, username, groupname)
}
