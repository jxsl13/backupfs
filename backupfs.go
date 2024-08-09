package backupfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
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
		append([]BackupFSOption{ /* default options that can be overwritten afterwards */ }, opts...)...,
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

	mu sync.Mutex
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

func (fsys *BackupFS) Map() (metadata map[string]fs.FileInfo) {

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

	fsys.baseInfos = m
}

func (fsys *BackupFS) MarshalJSON() ([]byte, error) {
	m := fsys.Map()

	fiMap := make(map[string]*fInfo, len(m))

	for path, fi := range m {
		if fi == nil {
			fiMap[path] = nil
			continue
		}

		fiMap[path] = toFInfo(path, fi)
	}

	return json.Marshal(fiMap)
}

func (fsys *BackupFS) UnmarshalJSON(data []byte) error {

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

func (fsys *BackupFS) ForceBackup(name string) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "force_backup", Err: err, Path: name}
		}
	}()

	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryRemoveBackup(resolvedName)
	if err != nil {
		return err
	}
	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	return nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (fsys *BackupFS) Create(name string) (_ File, err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "create", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return nil, err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return nil, err
	}

	// create or truncate file
	file, err := fsys.base.Create(resolvedName)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (fsys *BackupFS) Mkdir(name string, perm fs.FileMode) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "mkdir", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	err = fsys.base.Mkdir(resolvedName, perm)
	if err != nil {
		return err
	}
	return nil
}

// MkdirAll creates a directory path and all
// parents that does not exist yet.
func (fsys *BackupFS) MkdirAll(name string, perm fs.FileMode) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "mkdir_all", Path: name, Err: err}
		}
	}()

	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	err = fsys.base.MkdirAll(resolvedName, perm)
	if err != nil {
		return err
	}
	return nil
}

// OpenFile opens a file using the given flags and the given mode.
func (fsys *BackupFS) OpenFile(name string, flag int, perm fs.FileMode) (_ File, err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "open", Path: name, Err: err}
		}
	}()

	// read only operations do not require backups nor path resolution
	if flag == os.O_RDONLY {
		// in read only mode the perm is not used.
		f, err := fsys.base.OpenFile(name, os.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	// write operations require path resolution due to
	// potentially required backups
	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return nil, err
	}

	// not read only opening -> backup
	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return nil, err
	}

	file, err := fsys.base.OpenFile(resolvedName, flag, perm)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (fsys *BackupFS) Remove(name string) (err error) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	return fsys.remove(name)
}

func (fsys *BackupFS) remove(name string) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "remove", Path: name, Err: err}
		}
	}()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	err = fsys.base.Remove(resolvedName)
	if err != nil {
		return err
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
// not supported
func (fsys *BackupFS) RemoveAll(name string) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "remove_all", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	// does not exist or no access, nothing to do
	fi, err := fsys.Lstat(resolvedName)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		// if it's a file or a symlink, directly remove it
		err = fsys.remove(resolvedName)
		if err != nil {
			return err
		}
		return nil
	}

	resolvedDirPaths := make([]string, 0, 1)
	err = Walk(fsys.base, resolvedName, func(resolvedSubPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// initially we want to delete all files before we delete all of the directories
			// but we also want to keep track of all found directories in order not to walk the
			// dir tree again.
			resolvedDirPaths = append(resolvedDirPaths, resolvedSubPath)
			return nil
		}

		return fsys.remove(resolvedSubPath)
	})
	if err != nil {
		return err
	}

	// after deleting all of the files
	//now we want to sort all of the file paths from the most
	//nested file to the least nested file (count file path separators)
	sort.Sort(ByMostFilePathSeparators(resolvedDirPaths))

	for _, emptyDir := range resolvedDirPaths {
		err = fsys.remove(emptyDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// Rename renames a file.
func (fsys *BackupFS) Rename(oldname, newname string) (err error) {
	defer func() {
		if err != nil {
			err = &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedOldname, err := fsys.realPath(oldname)
	if err != nil {
		return err
	}

	resolvedNewname, newNameFound, err := fsys.realPathWithFound(newname)
	if err != nil {
		return err
	}

	if !newNameFound {
		// only make file known in case that it does not exist, otherwise
		// overwriting would return an error anyway.
		err = fsys.tryBackup(resolvedNewname)
		if err != nil {
			return err
		}

		// there either was no previous file to be backed up
		// but now we know that there was no file or there
		// was a target file that has to be backed up which was then backed up
		err = fsys.tryBackup(resolvedOldname)
		if err != nil {
			return err
		}
	}
	// in the else case Renaming to a file that already exists
	// the Rename call will return an error anyway, so we do not backup anything in that case.

	err = fsys.base.Rename(resolvedOldname, resolvedNewname)
	if err != nil {
		return err
	}
	return nil
}

// Chmod changes the mode of the named file to mode.
func (fsys *BackupFS) Chmod(name string, mode fs.FileMode) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "chmod", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	err = fsys.base.Chmod(resolvedName, mode)
	if err != nil {
		return err
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (fsys *BackupFS) Chown(name string, uid, gid int) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "chown", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	err = fsys.base.Chown(resolvedName, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (fsys *BackupFS) Chtimes(name string, atime, mtime time.Time) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "chown", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}
	err = fsys.base.Chtimes(resolvedName, atime, mtime)
	if err != nil {
		return err
	}

	return nil
}

// Symlink changes the access and modification times of the named file
func (fsys *BackupFS) Symlink(oldname, newname string) (err error) {
	defer func() {
		if err != nil {
			err = &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	// cannot resolve oldname because it is not touched and it may also contain relative paths
	resolvedNewname, err := fsys.realPath(newname)
	if err != nil {
		return err
	}

	// we only want to backup the newname,
	// as seemingly the new name is the target symlink location
	// the old file path should not have been modified

	// in case we fail to backup the symlink, we return an error
	err = fsys.tryBackup(resolvedNewname)
	if err != nil {
		return err
	}

	err = fsys.base.Symlink(oldname, resolvedNewname)
	if err != nil {
		return err
	}
	return nil
}

// Lchown does not fallback to chown. It does return an error in case that lchown cannot be called.
func (fsys *BackupFS) Lchown(name string, uid, gid int) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "lchown", Path: name, Err: err}
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	resolvedName, err := fsys.realPath(name)
	if err != nil {
		return err
	}

	//TODO: check if the owner stays equal and then backup the file if the owner changes
	// at this point we do modify the owner -> require backup
	err = fsys.tryBackup(resolvedName)
	if err != nil {
		return err
	}

	return fsys.base.Lchown(name, uid, gid)
}

// Rollback tries to rollback the backup back to the
// base system removing any new files for the base
// system and restoring any old files from the backup
// Best effort, any errors due to filesystem
// modification on the backup site are skipped
// This is a heavy weight operation which blocks the file system
// until the rollback is done.
func (fsys *BackupFS) Rollback() (multiErr error) {
	defer func() {
		if multiErr != nil {
			multiErr = errors.Join(ErrRollbackFailed, multiErr)
		}
	}()
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	var (
		// these file sneed to be removed in a certain order, so we keep track of them
		// from most nested to least nested files
		// can be any file type, dir, file, symlink
		removeBasePaths = make([]string, 0, 1)

		// these files also need to be restored in a certain order
		// from least nested to most nested
		restoreDirPaths     = make([]string, 0, 4)
		restoreFilePaths    = make([]string, 0, 4)
		restoreSymlinkPaths = make([]string, 0, 4)

		err    error
		exists bool
	)

	for path, info := range fsys.baseInfos {
		if info == nil {
			// file did not exist in the base filesystem at the point of
			// filesystem modification.
			_, exists, err = lexists(fsys.base, path)
			if err != nil {
				multiErr = errors.Join(
					multiErr,
					fmt.Errorf("failed to check whether file %s exists in base filesystem: %w", path, err),
				)
				continue
			}

			if exists {
				// we will need to delete this file
				removeBasePaths = append(removeBasePaths, path)
			}

			// case where file must be removed in base file system
			// finished
			continue
		} else if TrimVolume(path) == separator {
			// skip root directory from restoration
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

	err = fsys.tryRemoveBasePaths(removeBasePaths)
	if err != nil {
		multiErr = errors.Join(err)
	}

	err = fsys.tryRestoreDirPaths(restoreDirPaths)
	if err != nil {
		multiErr = errors.Join(multiErr, err)
	}

	err = fsys.tryRestoreFilePaths(restoreFilePaths)
	if err != nil {
		multiErr = errors.Join(multiErr, err)
	}

	err = fsys.tryRestoreSymlinkPaths(restoreSymlinkPaths)
	if err != nil {
		multiErr = errors.Join(multiErr, err)
	}

	// TODO: make this optional?: whether to delete the backup upon rollback

	// at this point we were able to restore all of the files
	// now we need to delete our backup
	err = fsys.tryRemoveBackupPaths("symlink", restoreSymlinkPaths)
	if err != nil {
		multiErr = errors.Join(multiErr, err)
	}

	// delete files before directories in order for directories to be empty
	err = fsys.tryRemoveBackupPaths("file", restoreFilePaths)
	if err != nil {
		multiErr = errors.Join(multiErr, err)
	}

	// best effort deletion of backup files
	// so we ignore the error
	// we only delete directories that we did create.
	// any user created content in directories is not touched

	err = fsys.tryRemoveBackupPaths("directory", restoreDirPaths)
	if err != nil {
		multiErr = errors.Join(multiErr, err)
	}

	// in case of a multiError we are not able to restore the previous state anyway
	// that is why we continue here to finish the rollback but at the same time inform
	// the user about potential errors along the way.

	// at this point we have successfully restored our backup and
	// removed all of the backup files and directories

	// now we can reset the internal data structure for book keeping of filesystem modifications
	fsys.baseInfos = make(map[string]fs.FileInfo, 1)
	return multiErr
}

func (fsys *BackupFS) tryRemoveBasePaths(removeBasePaths []string) (multiErr error) {
	var err error
	// remove files from most nested to least nested
	sort.Sort(ByMostFilePathSeparators(removeBasePaths))
	for _, remPath := range removeBasePaths {
		// remove all files that were not there before the backup.
		// ignore error, as this is a best effort restoration.
		// folders and files did not exist in the first place
		err = fsys.base.Remove(remPath)
		if err != nil {
			multiErr = errors.Join(
				multiErr,
				fmt.Errorf("failed to remove path in base filesystem %s: %w", remPath, err),
			)
		}
	}
	return multiErr
}

func (fsys *BackupFS) tryRemoveBackupPaths(fileType string, removeBackupPaths []string) (multiErr error) {
	var (
		err   error
		found bool
	)

	// remove files from most nested to least nested
	sort.Sort(ByMostFilePathSeparators(removeBackupPaths))
	for _, remPath := range removeBackupPaths {
		_, found, err = lexists(fsys.backup, remPath)
		if err != nil {
			multiErr = errors.Join(
				multiErr,
				fmt.Errorf("failed to check whether %s exists in backup filesystem %s: %w", fileType, remPath, err),
			)
			continue
		}

		if !found {
			// nothing to remove
			continue
		}

		// remove all files that were not there before the backup.
		// WARNING: do not change this to RemoveAll, as we do not want to remove user created content
		// in directories
		err = fsys.backup.Remove(remPath)
		if err != nil {
			multiErr = errors.Join(
				multiErr,
				fmt.Errorf("failed to remove %s in backup filesystem %s: %w", fileType, remPath, err),
			)
		}
	}
	return multiErr
}

func (fsys *BackupFS) tryRestoreDirPaths(restoreDirPaths []string) (multiErr error) {
	// in order to iterate over parent directories before child directories
	sort.Sort(ByLeastFilePathSeparators(restoreDirPaths))
	var err error
	for _, dirPath := range restoreDirPaths {
		// backup -> base filesystem
		err = copyDir(fsys.base, dirPath, fsys.baseInfos[dirPath])
		if err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

func (fsys *BackupFS) tryRestoreSymlinkPaths(restoreSymlinkPaths []string) (multiErr error) {
	// in this case it does not matter whether we sort the symlink paths or not
	// we prefer to sort them in order to see potential errors better
	sort.Strings(restoreSymlinkPaths)
	var err error
	for _, symlinkPath := range restoreSymlinkPaths {
		err = restoreSymlink(
			symlinkPath,
			fsys.baseInfos[symlinkPath],
			fsys.base,
			fsys.backup,
		)
		if err != nil {
			// in this case it might make sense to retry the rollback
			multiErr = errors.Join(multiErr, err)
		}
	}

	return multiErr
}

func (fsys *BackupFS) tryRestoreFilePaths(restoreFilePaths []string) (multiErr error) {
	// in this case it does not matter whether we sort the file paths or not
	// we prefer to sort them in order to see potential errors better
	sort.Strings(restoreFilePaths)
	var err error
	for _, filePath := range restoreFilePaths {
		err = restoreFile(filePath, fsys.baseInfos[filePath], fsys.base, fsys.backup)
		if err != nil {
			// in this case it might make sense to retry the rollback
			multiErr = errors.Join(multiErr, err)
		}
	}

	return multiErr
}

// returns the cleaned path
func (fsys *BackupFS) realPath(name string) (resolvedName string, err error) {
	return resolvePath(fsys, filepath.Clean(name))
}

func (fsys *BackupFS) realPathWithFound(name string) (resolvedName string, found bool, err error) {
	return resolvePathWithFound(fsys, filepath.Clean(name))
}

// keeps track of files in the base filesystem.
// Files are saved only once, any consecutive update is ignored.
func (fsys *BackupFS) setInfoIfNotAlreadySeen(path string, info fs.FileInfo) {
	_, found := fsys.baseInfos[path]
	if !found {
		fsys.baseInfos[path] = info
	}
}

func (fsys *BackupFS) alreadySeen(path string) bool {
	_, found := fsys.baseInfos[path]
	return found
}

func (fsys *BackupFS) alreadySeenWithInfo(path string) (fs.FileInfo, bool) {
	fi, found := fsys.baseInfos[path]
	return fi, found
}

func (fsys *BackupFS) tryRemoveBackup(resolvedName string) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "try_remove_backup", Path: resolvedName, Err: err}
		}
	}()

	if !fsys.alreadySeen(resolvedName) {
		// nothing to remove
		return nil
	}

	fi, err := fsys.backup.Lstat(resolvedName)
	if err != nil && !isNotFoundError(err) {
		return err
	}

	// file not found
	if fi == nil {
		// nothing to remove, except internal state if it exists

		delete(fsys.baseInfos, resolvedName)
		return nil
	}

	if !fi.IsDir() {
		// remove file or symlink
		err := fsys.backup.Remove(resolvedName)
		if err != nil {
			return err
		}
		// only delete from internal state
		// when file has been deleted
		// this allows to retry the deletion attempt
		delete(fsys.baseInfos, resolvedName)
		return nil
	}

	dirs := make([]string, 0)

	err = Walk(fsys.backup, resolvedName, func(path string, info fs.FileInfo, err error) (e error) {
		// and then check for error
		if err != nil {
			return err
		}

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
		// delete dirs and files from internal map
		// but only after re have removed the file successfully
		delete(fsys.baseInfos, path)
		return nil
	})
	if err != nil {
		return err
	}

	sort.Sort(ByMostFilePathSeparators(dirs))

	for _, dir := range dirs {
		// remove directory and potential content which should not be there
		err = fsys.backup.RemoveAll(dir)
		if err != nil {
			return err
		}

		// delete directory from internal
		// state only after it has been actually deleted
		delete(fsys.baseInfos, dir)
	}

	return nil
}

func (fsys *BackupFS) tryBackup(resolvedName string) (err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "try_backup", Path: resolvedName, Err: err}
		}
	}()

	info, needsBackup, err := fsys.backupRequired(resolvedName)
	if err != nil {
		return err
	}
	if !needsBackup {
		return nil
	}

	dirPath := resolvedName
	if !info.IsDir() {
		// is file, get dir
		dirPath = filepath.Dir(dirPath)
	}

	err = fsys.backupDirs(dirPath)
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
		sf, err := fsys.base.Open(resolvedName)
		if err != nil {
			return err
		}
		defer sf.Close()
		err = copyFile(fsys.backup, resolvedName, info, sf)
		if err != nil {
			return err
		}
		fsys.setInfoIfNotAlreadySeen(resolvedName, info)
		return nil
	case fileMode&os.ModeSymlink != 0:
		// symlink
		err = copySymlink(
			fsys.base,
			fsys.backup,
			resolvedName,
			info,
		)
		if err != nil {
			return err
		}
		fsys.setInfoIfNotAlreadySeen(resolvedName, info)
		return nil
	default:
		// unsupported file for backing up
		return nil
	}
}

// this method does not need to care about symlinks because it is passed a resolved path already which
// doe snot contain any directores that are symlinks
// resolvedDirPath MUST BE a directory
func (fsys *BackupFS) backupDirs(resolvedDirPath string) (err error) {
	_, err = IterateDirTree(resolvedDirPath, func(resolvedSubDirPath string) (bool, error) {
		// when the passed path is resolved, the subdir paths are implicitly also already resolved.

		// this should prevent infinite recursion due to circular symlinks
		fi, required, err := fsys.backupRequired(resolvedSubDirPath)
		if err != nil {
			return false, err
		}

		if !required {
			return true, nil
		}

		// is a directory, backup the directory
		err = copyDir(fsys.backup, resolvedSubDirPath, fi)
		if err != nil {
			return false, err
		}
		fsys.setInfoIfNotAlreadySeen(resolvedSubDirPath, fi)

		return true, nil
	})
	if err != nil {
		return &os.PathError{Op: "backup_dirs", Path: resolvedDirPath, Err: err}
	}
	return nil
}

// backupRequired checks whether a file that is about to be changed needs to be backed up.
// files that do not exist in the BackupFS need to be backed up.
// files that do exist in the BackupFS either as files or in the baseInfos map as non-existing files
// do not  need to be backed up (again)
func (fsys *BackupFS) backupRequired(resolvedName string) (info fs.FileInfo, required bool, err error) {

	info, found := fsys.alreadySeenWithInfo(resolvedName)
	if found {
		// base infos is the truth, if nothing is found, nothing needs to be backed up
		return info, false, nil
	}

	// fill fsys.baseInfos
	// of symlink, file & directory as well as their parent directories.
	info, err = fsys.Lstat(resolvedName)
	if isNotFoundError(err) {
		fsys.setInfoIfNotAlreadySeen(resolvedName, nil)
		// not found, no backup needed
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	return info, true, nil
}
