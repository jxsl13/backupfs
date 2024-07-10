package backupfs

import (
	"io/fs"
	"os"
	"time"
)

var (
	_ FS = (*OSFS)(nil)
)

func NewOSFS() OSFS {
	return OSFS{}
}

type OSFS struct{}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (OSFS) Create(name string) (File, error) {
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (OSFS) Mkdir(name string, perm fs.FileMode) error {
	err := os.Mkdir(name, perm)
	if err != nil {
		return err
	}
	return nil
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (OSFS) MkdirAll(path string, perm fs.FileMode) error {
	err := os.MkdirAll(path, perm)
	if err != nil {
		return err
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
func (OSFS) Open(name string) (File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenFile opens a file using the given flags and the given mode.
func (OSFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (OSFS) Remove(name string) error {
	err := os.Remove(name)
	if err != nil {
		return err
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (OSFS) RemoveAll(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}

// Rename renames a file.
func (OSFS) Rename(oldname, newname string) error {
	err := os.Rename(oldname, newname)
	if err != nil {
		return err
	}
	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (OSFS) Stat(name string) (fs.FileInfo, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

// The name of this FileSystem
func (OSFS) Name() string {
	return "OSFS"
}

// Chmod changes the mode of the named file to mode.
func (OSFS) Chmod(name string, mode fs.FileMode) error {
	err := os.Chmod(name, mode)
	if err != nil {
		return err
	}
	return nil
}

// Chown changes the uid and gid of the named file.
// TODO: improve windows support
func (OSFS) Chown(name string, uid, gid int) error {
	err := os.Chown(name, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (OSFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	err := os.Chtimes(name, atime, mtime)
	if err != nil {
		return err
	}
	return nil
}
func (OSFS) Lstat(name string) (fs.FileInfo, error) {
	fi, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	return fi, nil
}
func (OSFS) Symlink(oldname, newname string) error {
	err := os.Symlink(oldname, newname)
	if err != nil {
		return err
	}
	return nil
}
func (OSFS) Readlink(name string) (string, error) {
	link, err := os.Readlink(name)
	if err != nil {
		return "", err
	}
	return link, nil
}

// TODO: improve windows support
func (OSFS) Lchown(name string, uid int, gid int) error {
	err := os.Lchown(name, uid, gid)
	if err != nil {
		return err
	}
	return nil
}
