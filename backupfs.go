package backupfs

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/afero"
)

// assert interface implemented
var _ afero.Fs = (*BackupFs)(nil)

// File is implemented by the imported directory.
type File = afero.File

// New creates a new layered backup file system that backups files from fs to backup in case that an
// existing file in fs is about to be overwritten or removed.
func NewBackupFs(base, backup afero.Fs) *BackupFs {
	return &BackupFs{
		base:   base,
		backup: backup,

		// this map is needed in order to keep track of non existing files
		// consecutive changes might lead to file sbeing backe dup
		// that were never there before
		// but could have been written byus in the mean time.
		// without this structure we would never know where there was actually
		// no previous file to be backe dup.
		baseInfos: make(map[string]os.FileInfo),
	}
}

//
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

// updates
func (fs *BackupFs) setBaseInfoIfNotFound(path string, info os.FileInfo) {
	_, found := fs.baseInfos[path]
	if !found {
		fs.baseInfos[path] = info
	}
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
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

func (fs *BackupFs) backupRequired(path string) (info os.FileInfo, required bool, err error) {
	fs.mu.Lock()
	info, found := fs.baseInfos[path]
	if !found {
		// fill fs.baseInfos
		info, err = fs.stat(path)
		if err != nil {
			fs.mu.Unlock()
			return nil, false, err
		}
	}
	fs.mu.Unlock()

	// at this point info is either set by baseInfos or by fs.tat
	if info == nil {
		//actually no file expected at that location
		return nil, false, nil
	}

	// file found at base fs location

	// did we already backup that file?
	foundBackup, err := exists(fs.backup, path)
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
	name = cleanPath(name)

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

	err = iterateDirTree(dirPath, func(subDirPath string) error {
		fi, required, err := fs.backupRequired(subDirPath)
		if err != nil {
			return err
		}

		if !required {
			return nil
		}

		return copyDir(fs.backup, subDirPath, fi)
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
	return copyFile(fs.backup, name, info, sf)
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
	fi, err := fs.Stat(name)
	if err != nil {
		return nil, err
	}

	oldPerm := fi.Mode().Perm()
	if flag == os.O_RDONLY && oldPerm == perm {
		return fs.base.OpenFile(name, os.O_RDONLY, oldPerm)
	}

	// not read only opening -> backup
	err = fs.tryBackup(name)
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
func (s *BackupFs) RemoveAll(path string) error {
	return syscall.EPERM
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
