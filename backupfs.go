package backupfs

import (
	"os"
	"path/filepath"
	"strings"
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
func New(base, backup afero.Fs) *BackupFs {
	return &BackupFs{
		base:   base,
		backup: backup,
	}
}

//
type BackupFs struct {
	base   afero.Fs
	backup afero.Fs
}

// Returns true if the file is not in the backup
func (fs *BackupFs) isBaseFile(name string) (os.FileInfo, bool, error) {
	// file exists in backup ->
	if _, err := fs.backup.Stat(name); err == nil {
		return nil, false, nil
	}
	stat, err := fs.base.Stat(name)
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if oerr.Err == os.ErrNotExist || oerr.Err == syscall.ENOENT || oerr.Err == syscall.ENOTDIR {
				return nil, false, nil
			}
		}
		if err == syscall.ENOENT {
			return nil, false, nil
		}
	}
	return stat, true, err
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *BackupFs) Create(name string) (File, error) {
	_, isBf, err := s.isBaseFile(name)
	if err != nil {
		return nil, err
	}
	// file does not exist in base layer
	if !isBf {
		return s.backup.Create(name)
	}

	// file does exist in base layer, move to backup layer
	err = copyToLayer(s.base, s.backup, name)
	if err != nil {
		return nil, err
	}

	// create or truncate file
	return s.base.Create(name)
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *BackupFs) Mkdir(name string, perm os.FileMode) error {
	return s.base.Mkdir(name, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *BackupFs) MkdirAll(path string, perm os.FileMode) error {
	name := strings.TrimRight(path, string(filepath.Separator))

	create := false
	lastIndex := 0
	for i, r := range name {
		if i == 0 && r == filepath.Separator {
			continue
		}
		create = false

		if r == filepath.Separator {
			create = true
			lastIndex = i - 1
		} else if i == len(name)-1 {
			create = true
			lastIndex = i
		}

		if create {
			// /path -> /path/subpath -> /path/subpath/subsubpath etc.
			dirPath := name[:lastIndex]

			exists, err := dirExists(s.backup, dirPath)
			if err != nil {
				return err
			}
			if exists {
				// dir exists in backup already
				// no need to create
				continue
			}

			// does not exist in backup
			// get permissions
			baseDir, err := s.base.Stat(dirPath)
			if err != nil {
				return err
			}

			// create dir
			err = s.backup.Mkdir(dirPath, baseDir.Mode().Perm())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *BackupFs) Open(name string) (File, error) {
	return s.base.Open(name)
}

// OpenFile opens a file using the given flags and the given mode.
func (s *BackupFs) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	baseStat, isBf, err := s.isBaseFile(name)
	if err != nil {
		return nil, err
	}
	// file does not exist in base layer
	if !isBf {
		return s.base.OpenFile(name, flag, perm)
	}

	oldPerm := baseStat.Mode()
	// file is being opened in read only mode
	if flag == os.O_RDONLY && oldPerm == perm {
		return s.base.OpenFile(name, os.O_RDONLY, oldPerm)
	}

	// file does exist in base layer, move to backup layer
	err = copyToLayer(s.base, s.backup, name)
	if err != nil {
		return nil, err
	}

	return s.base.OpenFile(name, flag, perm)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *BackupFs) Remove(name string) error {
	_, isBf, err := s.isBaseFile(name)
	if err != nil {
		return err
	}

	if !isBf {
		return s.base.Remove(name)
	}

	// file does exist in base layer, move to backup layer
	err = copyToLayer(s.base, s.backup, name)
	if err != nil {
		return err
	}
	return s.base.Remove(name)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *BackupFs) RemoveAll(path string) error {
	//return s.base.RemoveAll(path)
	return syscall.EPERM
}

// Rename renames a file.
func (s *BackupFs) Rename(oldname, newname string) error {
	//return s.base.Rename(oldname, newname)
	return syscall.EPERM
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *BackupFs) Stat(name string) (os.FileInfo, error) {
	return s.base.Stat(name)
}

// The name of this FileSystem
func (s *BackupFs) Name() string {
	return "BackupFs"
}

// Chmod changes the mode of the named file to mode.
func (s *BackupFs) Chmod(name string, mode os.FileMode) error {
	//return s.base.Chmod(name, mode)#
	return syscall.EPERM
}

// Chown changes the uid and gid of the named file.
func (s *BackupFs) Chown(name string, uid, gid int) error {
	//return s.base.Chown(name, uid, gid)
	return syscall.EPERM
}

//Chtimes changes the access and modification times of the named file
func (s *BackupFs) Chtimes(name string, atime, mtime time.Time) error {
	//return s.base.Chtimes(name, atime, mtime)
	return syscall.EPERM
}

// LstatIfPossible will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *BackupFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	if l, ok := s.base.(afero.Lstater); ok {
		// implements interface
		return l.LstatIfPossible(name)
	}

	// does not implement lstat, fallback to stat
	fi, err := s.base.Stat(name)
	return fi, false, err

}
