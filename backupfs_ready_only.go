package backupfs

import (
	"io/fs"
	"os"
)

func (fsys *BackupFS) Lstat(name string) (fi fs.FileInfo, err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "lstat", Path: name, Err: err}
		}
	}()

	return fsys.base.Lstat(name)
}

// Stat returns a FileInfo describing the named file, or an error, if any happens.
// Stat only looks at the base filesystem and returns the stat of the files at the specified path
func (fsys *BackupFS) Stat(name string) (_ fs.FileInfo, err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "stat", Path: name, Err: err}
		}
	}()

	return fsys.base.Stat(name)
}

func (fsys *BackupFS) Readlink(name string) (_ string, err error) {
	defer func() {
		if err != nil {
			err = &os.PathError{Op: "readlink", Path: name, Err: err}
		}
	}()

	path, err := fsys.base.Readlink(name)
	if err != nil {
		return "", err
	}
	return path, nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (fsys *BackupFS) Open(name string) (File, error) {
	return fsys.OpenFile(name, os.O_RDONLY, 0)
}
