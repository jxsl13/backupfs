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
)

var (
	// assert interfaces implemented
	_ FS = (*BackupFS)(nil)

	// ErrRollbackFailed is returned when the rollback fails due to e.g. network problems.
	// when this error is returned it might make sense to retry the rollback
	ErrRollbackFailed = errors.New("rollback failed")
)

// Options in order to manipulate the behavior of the BackupFS
type BackupFSOption func(*backupFSOptions)

// New creates a new layered backup file system that backups files from the OS filesystem the backup location in case that an
// existing file in the OS filesystem is about to be overwritten or removed.
// The backup location is hidden from the user's access in order to prevent infinite backup recursions.
// The returned BackupFS is OS-independent and can also be used with Windows paths.
func New(backupLocation string, opts ...BackupFSOption) *BackupFS {
	return NewWithFS(NewOSFS(), backupLocation, opts...)
}

// NewWithFS creates a new layered backup file system that backups files from fs to backup in case that an
// existing file in fs is about to be overwritten or removed.
// The backup location is hidden from the user's access i norder to prevent infinite backup recursions.
// The returned BackupFS is OS-independent and can also be used with Windows paths.
func NewWithFS(baseFS FS, backupLocation string, opts ...BackupFSOption) *BackupFS {
	fsys := NewBackupFS(
		NewHiddenFS(baseFS, backupLocation),
		NewPrefixFS(baseFS, backupLocation),
		// put our default option first in order for it to be overwritable later
		append([]BackupFSOption{WithVolumePaths(true)}, opts...)...,
	)
	return fsys
}

// NewBackupFS creates a new layered backup file system that backups files from fs to backup in case that an
// existing file in fs is about to be overwritten or removed.
func NewBackupFS(base, backup FS, opts ...BackupFSOption) *BackupFS {
	opt := &backupFSOptions{}

	for _, o := range opts {
		o(opt)
	}

	bfsys := &BackupFS{
		windowsVolumePaths: opt.allowWindowsVolumePaths,
		base:               base,
		backup:             backup,

		// this map is needed in order to keep track of non existing files
		// consecutive changes might lead to files being backed up
		// that were never there before
		// but could have been written by us in the mean time.
		// without this structure we would never know whether there was actually
		// no previous file to be backed up.
		baseInfos: make(map[string]fs.FileInfo),
	}
	return bfsys
}

// BackupFS is a file system abstraction that takes two underlying filesystems.
// One filesystem that is is being used to read and write files and a second filesystem
// which is used as backup target in case that a file of the base filesystem is about to be
// modified.
type BackupFS struct {
	// base filesystem which may be overwritten
	base FS
	// any initially overwritten file will be backed up to this filesystem
	backup FS

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

// BaseFS returns the fs layer that is being written to
func (fsys *BackupFS) BaseFS() FS {
	return fsys.base
}

// BackupFS returns the fs layer that is used to store the backups
func (fsys *BackupFS) BackupFS() FS {
	return fsys.backup
}

// The name of this FileSystem
func (fsys *BackupFS) Name() string {
	return "BackupFS"
}

// Rollback tries to rollback the backup back to the
// base system removing any new files for the base
// system and restoring any old files from the backup
// Best effort, any errors due to filesystem
// modification on the backup site are skipped
// This is a heavy weight operation which blocks the file system
// until the rollback is done.
func (fsys *BackupFS) Rollback() error {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	// these file sneed to be removed in a certain order, so we keep track of them
	// from most nested to least nested files
	removeBaseFiles := make([]string, 0, 1)

	// these files also need to be restored in a certain order
	// from least nested to most nested
	restoreDirPaths := make([]string, 0, 4)
	restoreFilePaths := make([]string, 0, 4)
	restoreSymlinkPaths := make([]string, 0, 4)

	for path, info := range fsys.baseInfos {
		if info == nil {
			// file did not exist in the base filesystem at the point of
			// filesystem modification.
			exists, err := lExists(fsys.base, path)
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
	sort.Sort(byMostFilePathSeparators(removeBaseFiles))
	for _, remPath := range removeBaseFiles {
		// remove all files that were not there before the backup.
		// ignore error, as this is a best effort restoration.
		_ = fsys.base.Remove(remPath)
	}

	// in order to iterate over parent directories before child directories
	sort.Sort(byLeastFilePathSeparators(restoreDirPaths))

	for _, dirPath := range restoreDirPaths {
		// backup -> base filesystem
		err := copyDir(fsys.base, dirPath, fsys.baseInfos[dirPath])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRollbackFailed, err)
		}
	}

	// in this case it does not matter whether we sort the file paths or not
	// we prefer to sort them in order to see potential errors better
	sort.Strings(restoreFilePaths)

	for _, filePath := range restoreFilePaths {
		err := restoreFile(filePath, fsys.baseInfos[filePath], fsys.base, fsys.backup)
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
		err := restoreSymlink(
			symlinkPath,
			fsys.baseInfos[symlinkPath],
			fsys.base,
			fsys.backup,
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
		_ = fsys.backup.Remove(symlinkPath)
	}

	// delete all files first
	for _, filePath := range restoreFilePaths {
		// best effort deletion of backup files
		// so we ignore the error
		_ = fsys.backup.Remove(filePath)
	}

	// we want to delete all of the backed up folders from
	// the most nested child directories to the least nested parent directories.
	sort.Sort(byMostFilePathSeparators(restoreDirPaths))

	// delete all files first
	for _, dirPath := range restoreDirPaths {
		// best effort deletion of backup files
		// so we ignore the error
		// we only delete directories that we did create.
		// any user created content in directories is not touched
		_ = fsys.backup.Remove(dirPath)
	}

	// at this point we have successfully restored our backup and
	// removed all of the backup files and directories

	// now we can reset the internal data structure for book keeping of filesystem modifications
	fsys.baseInfos = make(map[string]fs.FileInfo)
	return nil
}

func (fsys *BackupFS) Map() (metadata map[string]fs.FileInfo) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	m := make(map[string]fs.FileInfo, len(fsys.baseInfos))
	for path, info := range fsys.baseInfos {
		if info == nil {
			m[path] = nil // nil w/o type information is needed here
			continue
		}

		m[path] = toFInfo(path, info)
	}
	return m
}

func (fsys *BackupFS) SetMap(metadata map[string]fs.FileInfo) {

	// clone state
	m := make(map[string]fs.FileInfo, len(metadata))
	for path, info := range metadata {
		if info == nil {
			m[path] = nil
			continue
		}
		m[path] = info
	}

	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	fsys.baseInfos = m
}

func toFInfo(path string, fi fs.FileInfo) *fInfo {
	return &fInfo{
		FileName:    filepath.ToSlash(path),
		FileMode:    uint32(fi.Mode()),
		FileModTime: fi.ModTime().UnixNano(),
		FileSize:    fi.Size(),
		FileUid:     toUID(fi),
		FileGid:     toGID(fi),
	}
}

type fInfo struct {
	FileName    string `json:"name"`
	FileMode    uint32 `json:"mode"`
	FileModTime int64  `json:"mod_time"`
	FileSize    int64  `json:"size"`
	FileUid     int    `json:"uid"`
	FileGid     int    `json:"gid"`
}

func (fi *fInfo) Name() string {
	return path.Base(fi.FileName)
}
func (fi *fInfo) Size() int64 {
	return fi.FileSize
}
func (fi *fInfo) Mode() fs.FileMode {
	return fs.FileMode(fi.FileMode)
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

func (fsys *BackupFS) MarshalJSON() ([]byte, error) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	fiMap := make(map[string]*fInfo, len(fsys.baseInfos))

	for path, fi := range fsys.baseInfos {
		if fi == nil {
			fiMap[path] = nil
			continue
		}

		fiMap[path] = toFInfo(path, fi)
	}

	return json.Marshal(fiMap)
}

func (fsys *BackupFS) UnmarshalJSON(data []byte) error {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	fiMap := make(map[string]*fInfo)

	err := json.Unmarshal(data, &fiMap)
	if err != nil {
		return err
	}

	fsys.baseInfos = make(map[string]fs.FileInfo, len(fiMap))
	for k, v := range fiMap {
		if v == nil {
			// required, otherwise the value cannot be checked whethe rit's nil or not
			// due to the additional type information of k, which is of type *fInfo
			fsys.baseInfos[k] = nil
			continue
		}
		fsys.baseInfos[k] = v
	}

	return nil
}

// returns the cleaned path
func (fsys *BackupFS) realPath(name string) (path string, err error) {
	// check path for being an absolute windows path
	if !fsys.windowsVolumePaths && runtime.GOOS == "windows" && filepath.IsAbs(name) {
		// On Windows a common mistake would be to provide an absolute OS path
		// We could strip out the base part, but that would not be very portable.

		return name, fs.ErrNotExist
	}

	return filepath.Clean(name), nil
}

// keeps track of files in the base filesystem.
// Files are saved only once, any consecutive update is ignored.
func (fsys *BackupFS) setBaseInfoIfNotFound(path string, info fs.FileInfo) {
	_, found := fsys.baseInfos[path]
	if !found {
		fsys.baseInfos[path] = info
	}
}

// alreadyFoundBaseInfo returns true when we already visited this path.
// This is a helper function in order to NOT call the Stat method of the baseFS
// an unnecessary amount of times for filepath sub directories when we can just lookup
// the information in out internal filepath map
func (fsys *BackupFS) alreadyFoundBaseInfo(path string) bool {
	_, found := fsys.baseInfos[path]
	return found
}

// Stat returns a FileInfo describing the named file, or an error, if any happens.
// Stat only looks at the base filesystem and returns the stat of the files at the specified path
func (fsys *BackupFS) Stat(name string) (fs.FileInfo, error) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	return fsys.stat(name)
}

func (fsys *BackupFS) stat(name string) (fs.FileInfo, error) {
	name, err := fsys.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	// we want to check all parent directories before we check the actual file.
	// in order to keep track of their state as well.
	// /root -> /root/sub/ -> /root/sub/sub1
	// iterate parent directories and keep track of their initial state.
	_, _ = IterateDirTree(filepath.Dir(name), func(subdirPath string) (bool, error) {
		if fsys.alreadyFoundBaseInfo(subdirPath) {
			return true, nil
		}

		// only in case that we have not yet visited one of the subdirs already,
		// only then fetch the file information from the underlying baseFS
		// we do want to ignore errors as this is only for keeping track of subdirectories
		// TODO: in some weird scenario it might be possible for this value to be a symlink
		// instead of a directory
		_, err := fsys.trackedLstat(subdirPath)
		if err != nil {
			// in case of an error we want to fail fast
			return false, nil
		}
		return true, nil
	})

	return fsys.trackedStat(name)
}

// trackedStat is the tracked variant of Stat that is called on the underlying base FS
func (fsys *BackupFS) trackedStat(name string) (fs.FileInfo, error) {
	fi, err := fsys.base.Stat(name)

	// keep track of initial
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if oerr.Err == fs.ErrNotExist || oerr.Err == syscall.ENOENT || oerr.Err == syscall.ENOTDIR {

				fsys.setBaseInfoIfNotFound(name, nil)
				return nil, err
			}
		}
		if err == syscall.ENOENT {
			fsys.setBaseInfoIfNotFound(name, nil)
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	fsys.setBaseInfoIfNotFound(name, fi)
	return fi, nil
}

// backupRequired checks whether a file that is about to be changed needs to be backed up.
// files that do not exist in the BackupFS need to be backed up.
// files that do exist in the BackupFS either as files or in the baseInfos map as non-existing files
// do not  need to be backed up (again)
func (fsys *BackupFS) backupRequired(path string) (info fs.FileInfo, required bool, err error) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	info, found := fsys.baseInfos[path]
	if !found {
		// fill fsys.baseInfos
		// of symlink, file & directory as well as their parent directories.
		info, err = fsys.lstat(path)
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
	foundBackup, err := lExists(fsys.backup, path)
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

func (fsys *BackupFS) ForceBackup(name string) (err error) {
	name, err = fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "force_backup", Err: fmt.Errorf("failed to clean path: %w", err), Path: name}
	}

	err = fsys.tryRemoveBackup(name)
	if err != nil {
		return &os.PathError{Op: "force_backup", Err: fmt.Errorf("failed to remove backup: %w", err), Path: name}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "force_backup", Err: fmt.Errorf("backup failed: %w", err), Path: name}
	}
	return nil
}

func (fsys *BackupFS) tryRemoveBackup(name string) (err error) {
	_, needsBackup, err := fsys.backupRequired(name)
	if err != nil {
		return err
	}
	// there is no backup
	if needsBackup {
		return nil
	}

	fi, err := fsys.backup.Lstat(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// file not found
	if fi == nil {
		// nothing to remove, except internal state if it exists
		fsys.mu.Lock()
		defer fsys.mu.Unlock()

		delete(fsys.baseInfos, name)
		return nil
	}

	if !fi.IsDir() {
		defer func() {
			if err == nil {
				// only delete from internal state
				// when file has been deleted
				// this allows to retry the deletion attempt
				fsys.mu.Lock()
				delete(fsys.baseInfos, name)
				fsys.mu.Unlock()
			}
		}()
		// remove file/symlink
		err := fsys.backup.Remove(name)
		if err != nil {
			return err
		}
		return nil
	}

	dirs := make([]string, 0)

	err = Walk(fsys.backup, name, func(path string, info fs.FileInfo, err error) (e error) {
		// and then check for error
		if err != nil {
			return err
		}

		defer func() {
			if e == nil {
				// delete dirs and files from internal map
				// but only after re have removed the file successfully
				fsys.mu.Lock()
				delete(fsys.baseInfos, path)
				fsys.mu.Unlock()
			}
		}()

		if info.IsDir() {
			// keep track of dirs
			dirs = append(dirs, path)
			return nil
		}

		// delete files
		err = fsys.backup.Remove(path)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	sort.Sort(byMostFilePathSeparators(dirs))

	for _, dir := range dirs {
		err = fsys.backup.RemoveAll(dir)
		if err != nil {
			return err
		}

		// delete directory from internal
		// state only after it has been actually deleted
		fsys.mu.Lock()
		delete(fsys.baseInfos, dir)
		fsys.mu.Unlock()
	}

	return nil
}

func (fsys *BackupFS) tryBackup(name string) (err error) {

	info, needsBackup, err := fsys.backupRequired(name)
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

	_, err = IterateDirTree(dirPath, func(subDirPath string) (bool, error) {
		fi, required, err := fsys.backupRequired(subDirPath)
		if err != nil {
			return false, err
		}

		if !required {
			return true, nil
		}

		err = copyDir(fsys.backup, subDirPath, fi)
		if err != nil {
			return false, err
		}
		return true, nil
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
		sf, err := fsys.base.Open(name)
		if err != nil {
			return err
		}
		defer sf.Close()
		return copyFile(fsys.backup, name, info, sf)

	case fileMode&os.ModeSymlink != 0:
		// symlink
		return copySymlink(
			fsys.base,
			fsys.backup,
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
func (fsys *BackupFS) Create(name string) (File, error) {
	name, err := fsys.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}
	// create or truncate file
	file, err := fsys.base.Create(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: fmt.Errorf("create failed: %w", err)}
	}
	return file, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (fsys *BackupFS) Mkdir(name string, perm fs.FileMode) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fsys.base.Mkdir(name, perm)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: fmt.Errorf("mkdir failed: %w", err)}
	}
	return nil
}

// MkdirAll creates a directory path and all
// parents that does not exist yet.
func (fsys *BackupFS) MkdirAll(name string, perm fs.FileMode) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fsys.base.MkdirAll(name, perm)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: fmt.Errorf("mkdir_all failed: %w", err)}
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (fsys *BackupFS) Open(name string) (File, error) {
	return fsys.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (fsys *BackupFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	name, err := fsys.realPath(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	if flag == os.O_RDONLY {
		// in read only mode the perm is not used.
		f, err := fsys.base.OpenFile(name, os.O_RDONLY, 0)
		if err != nil {
			return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("open failed: %w", err)}
		}
		return f, nil
	}

	// not read only opening -> backup
	err = fsys.tryBackup(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	file, err := fsys.base.OpenFile(name, flag, perm)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("open failed: %w", err)}
	}
	return file, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (fsys *BackupFS) Remove(name string) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fsys.base.Remove(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: fmt.Errorf("remove failed: %w", err)}
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
// not supported
func (fsys *BackupFS) RemoveAll(name string) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	// does not exist or no access, nothing to do
	fi, err := fsys.Lstat(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// if it's a file or a symlink, directly remove it
	if !fi.IsDir() {
		err = fsys.Remove(name)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("remove failed: %w", err)}
		}
		return nil
	}

	directoryPaths := make([]string, 0, 1)

	err = Walk(fsys.base, name, func(path string, info fs.FileInfo, err error) error {
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

		return fsys.Remove(path)
	})

	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: fmt.Errorf("walkdir or remove failed: %w", err)}
	}

	// after deleting all of the files
	//now we want to sort all of the file paths from the most
	//nested file to the least nested file (count file path separators)
	sort.Sort(byMostFilePathSeparators(directoryPaths))

	for _, path := range directoryPaths {
		err = fsys.Remove(path)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: err}
		}
	}

	return nil
}

// Rename renames a file.
func (fsys *BackupFS) Rename(oldname, newname string) error {
	newname, err := fsys.realPath(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: fmt.Errorf("failed to clean newname: %w", err)}
	}

	oldname, err = fsys.realPath(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("failed to clean oldname: %w", err)}
	}

	// make target file known
	err = fsys.tryBackup(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: fmt.Errorf("failed to backup newname: %w", err)}
	}

	// there either was no previous file to be backed up
	// but now we know that there was no file or there
	// was a target file that has to be backed up which was then backed up

	err = fsys.tryBackup(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("failed to backup oldname: %w", err)}
	}

	err = fsys.base.Rename(oldname, newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: fmt.Errorf("renaming failed: %w", err)}
	}
	return nil
}

// Chmod changes the mode of the named file to mode.
func (fsys *BackupFS) Chmod(name string, mode fs.FileMode) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("failed to get clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fsys.base.Chmod(name, mode)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: fmt.Errorf("chmod failed: %w", err)}
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (fsys *BackupFS) Chown(name string, uid, gid int) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	err = fsys.base.Chown(name, uid, gid)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: fmt.Errorf("chown failed: %w", err)}
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (fsys *BackupFS) Chtimes(name string, atime, mtime time.Time) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}
	err = fsys.base.Chtimes(name, atime, mtime)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: fmt.Errorf("chtimes failed: %w", err)}
	}
	return nil
}

func (fsys *BackupFS) Lstat(name string) (fs.FileInfo, error) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	return fsys.lstat(name)
}

// lstat has the same logic as stat
func (fsys *BackupFS) lstat(name string) (fs.FileInfo, error) {
	name, err := fsys.realPath(name)
	if err != nil {
		return nil, err
	}

	// we want to check all parent directories before we check the actual file.
	// in order to keep track of their state as well.
	// /root -> /root/sub/ -> /root/sub/sub1
	// iterate parent directories and keep track of their initial state.
	_, _ = IterateDirTree(filepath.Dir(name), func(subdirPath string) (bool, error) {
		if fsys.alreadyFoundBaseInfo(subdirPath) {
			return true, nil
		}

		// only in the case that we do not know the subdirectory already
		// we do want to track the initial state of the sub directory.
		// if it does not exist, it should not exist
		_, err = fsys.trackedLstat(subdirPath)
		if err != nil {
			// in case of an error we want to fail fast
			return false, nil
		}
		return true, nil
	})

	// check is actual file exists
	fi, err := fsys.trackedLstat(name)
	if err != nil {
		return nil, err
	}

	return fi, nil
}

// trackedLstat has the same logic as trackedStat but it uses Lstat instead, in case that is possible.
func (fsys *BackupFS) trackedLstat(name string) (fs.FileInfo, error) {

	fi, err := fsys.base.Lstat(name)

	// keep track of initial
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if oerr.Err == fs.ErrNotExist || oerr.Err == syscall.ENOENT || oerr.Err == syscall.ENOTDIR {
				// file or symlink does not exist
				fsys.setBaseInfoIfNotFound(name, nil)
				return nil, err
			}
		} else if err == syscall.ENOENT {
			// file or symlink does not exist
			fsys.setBaseInfoIfNotFound(name, nil)
			return nil, err
		}
		return nil, err
	}

	fsys.setBaseInfoIfNotFound(name, fi)
	return fi, nil
}

// Symlink changes the access and modification times of the named file
func (fsys *BackupFS) Symlink(oldname, newname string) error {
	oldname, err := fsys.realPath(oldname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to clean oldname: %w", err)}
	}

	newname, err = fsys.realPath(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to clean newname: %w", err)}
	}

	// we only want to backup the newname,
	// as seemingly the new name is the target symlink location
	// the old file path should not have been modified

	// in case we fail to backup the symlink, we return an error
	err = fsys.tryBackup(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("failed to backup newname: %w", err)}
	}

	err = fsys.base.Symlink(oldname, newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("symlink failed: %w", err)}
	}
	return nil
}

func (fsys *BackupFS) Readlink(name string) (string, error) {
	name, err := fsys.realPath(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	path, err := fsys.base.Readlink(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fmt.Errorf("readlink failed: %w", err)}
	}
	return path, nil
}

// Lchown does not fallback to chown. It does return an error in case that lchown cannot be called.
func (fsys *BackupFS) Lchown(name string, uid, gid int) error {
	name, err := fsys.realPath(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: fmt.Errorf("failed to clean path: %w", err)}
	}

	//TODO: check if the owner stays equal and then backup the file if the owner changes
	// at this point we do modify the owner -> require backup
	err = fsys.tryBackup(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: fmt.Errorf("failed to backup path: %w", err)}
	}

	return fsys.base.Lchown(name, uid, gid)
}
