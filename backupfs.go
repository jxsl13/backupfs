package backupfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
)

// assert interface implemented
var (
	_ afero.Fs = (*BackupFs)(nil)

	// ErrRollbackFailed is returned when the rollback fails due to e.g. network problems.
	// when this error is returned it might make sense to retry the rollback
	ErrRollbackFailed = errors.New("rollback failed")
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

	for path, info := range fs.baseInfos {
		if info == nil {
			// file did not exist in the base filesystem at the point of
			// filesystem modification.
			exists, err := internal.Exists(fs.base, path)
			if err == nil && exists {
				// we will need to delete this file
				removeBaseFiles = append(removeBaseFiles, path)
			}

			// case where file must be removed in base file system
			// finished
			continue
		}

		// file did exist in base filesystem, so we need to restore it from the backup
		if info.IsDir() {
			restoreDirPaths = append(restoreDirPaths, path)
		} else {
			restoreFilePaths = append(restoreFilePaths, path)
		}
	}

	// remove files from most nested to least nested
	sort.Sort(internal.ByMostFilePathSeparators(removeBaseFiles))
	for _, remPath := range removeBaseFiles {
		// remove all files that were not ther ebefor ethe backup.
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
	// we prefer to sort it in order to see potential errors better
	sort.Strings(restoreFilePaths)

	for _, filePath := range restoreFilePaths {
		err := internal.RestoreFile(filePath, fs.baseInfos[filePath], fs.base, fs.backup)
		if err != nil {
			// in this case it might make sense to retry the rollback
			return fmt.Errorf("%w: %v", ErrRollbackFailed, err)
		}
	}
	return nil
}

// keeps track of files in the base filesystem.
// Files are saved only once, any consecutive update is ignored.
func (fs *BackupFs) setBaseInfoIfNotFound(path string, info os.FileInfo) {
	_, found := fs.baseInfos[path]
	if !found {
		fs.baseInfos[path] = info
	}
}

// Stat returns a FileInfo describing the named file, or an error, if any happens.
// Stat only looks at the base filesystem and returns the stat of the files at the specified path
func (fs *BackupFs) Stat(name string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.stat(name)
}

func (fs *BackupFs) stat(name string) (os.FileInfo, error) {
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
	info, found := fs.baseInfos[path]
	if !found {
		defer fs.mu.Unlock()
		// fill fs.baseInfos
		info, err = fs.stat(path)
		if err == nil {
			return info, true, nil
		}
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	fs.mu.Unlock()

	// at this point info is either set by baseInfos or by fs.tat
	if info == nil {
		//actually no file expected at that location
		return nil, false, nil
	}

	// file found at base fs location

	// did we already backup that file?
	foundBackup, err := internal.Exists(fs.backup, path)
	if err != nil {
		return nil, false, err
	}

	if foundBackup {
		// no need to backup, as we already backe dup the file
		return nil, false, nil
	}

	// backup is needed
	return info, true, nil
}

func (fs *BackupFs) tryBackup(name string) error {

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

	if info.IsDir() {
		// name was a dir path, we are finished
		return nil
	}

	// name was a path to a file
	// create the file
	sf, err := fs.base.Open(name)
	if err != nil {
		return err
	}
	defer sf.Close()
	return internal.CopyFile(fs.backup, name, info, sf)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (fs *BackupFs) Create(name string) (File, error) {
	err := fs.tryBackup(name)
	if err != nil {
		return nil, err
	}
	// create or truncate file
	return fs.base.Create(name)
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (fs *BackupFs) Mkdir(name string, perm os.FileMode) error {
	err := fs.tryBackup(name)
	if err != nil {
		return err
	}
	return fs.base.Mkdir(name, perm)
}

// MkdirAll creates a directory path and all
// parents that does not exist yet.
func (fs *BackupFs) MkdirAll(name string, perm os.FileMode) error {
	err := fs.tryBackup(name)
	if err != nil {
		return err
	}

	return fs.base.MkdirAll(name, perm)
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (fs *BackupFs) Open(name string) (File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (fs *BackupFs) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	if flag == os.O_RDONLY {
		// in read only mode the perm is not used.
		return fs.base.OpenFile(name, os.O_RDONLY, 0)
	}

	// not read only opening -> backup
	err := fs.tryBackup(name)
	if err != nil {
		return nil, err
	}

	return fs.base.OpenFile(name, flag, perm)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (fs *BackupFs) Remove(name string) error {
	err := fs.tryBackup(name)
	if err != nil {
		return err
	}

	return fs.base.Remove(name)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
// not supported
func (s *BackupFs) RemoveAll(name string) error {
	fi, err := s.Stat(name)
	if err != nil {
		return err
	}

	// if it's a file, directly remove it
	if !fi.IsDir() {
		return s.Remove(name)
	}

	directoryPaths := make([]string, 0, 1)

	err = afero.Walk(s.base, name, func(path string, info fs.FileInfo, err error) error {
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

		return s.Remove(path)
	})

	if err != nil {
		return err
	}

	// after deleting all of the files
	//now we want to sort all of the file paths from the most
	//nested file to the least nested file (count file path separators)
	sort.Sort(internal.ByMostFilePathSeparators(directoryPaths))

	for _, path := range directoryPaths {
		err = s.Remove(path)
		if err != nil {
			return err
		}
	}

	return nil
}

// Rename renames a file.
func (fs *BackupFs) Rename(oldname, newname string) error {
	// make target file known
	err := fs.tryBackup(newname)
	if err != nil {
		return err
	}

	// there either was no previous file to be backed up
	// but now we know that there was no file or there
	// was a target file that has to be backed up which was then backed up

	err = fs.tryBackup(oldname)
	if err != nil {
		return err
	}

	return fs.base.Rename(oldname, newname)
}

// Chmod changes the mode of the named file to mode.
func (fs *BackupFs) Chmod(name string, mode os.FileMode) error {
	err := fs.tryBackup(name)
	if err != nil {
		return err
	}

	return fs.base.Chmod(name, mode)
}

// Chown changes the uid and gid of the named file.
func (fs *BackupFs) Chown(name string, uid, gid int) error {
	err := fs.tryBackup(name)
	if err != nil {
		return err
	}

	return fs.base.Chown(name, uid, gid)
}

//Chtimes changes the access and modification times of the named file
func (fs *BackupFs) Chtimes(name string, atime, mtime time.Time) error {
	err := fs.tryBackup(name)
	if err != nil {
		return err
	}

	return fs.base.Chtimes(name, atime, mtime)
}
