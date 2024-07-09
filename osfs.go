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
	return os.Create(name)
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (OSFS) Mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(name, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (OSFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Open opens a file, returning it or an error, if any happens.
func (OSFS) Open(name string) (File, error) {
	return os.Open(name)
}

// OpenFile opens a file using the given flags and the given mode.
func (OSFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (OSFS) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (OSFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename renames a file.
func (OSFS) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (OSFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// The name of this FileSystem
func (OSFS) Name() string {
	return "OSFS"
}

// Chmod changes the mode of the named file to mode.
func (OSFS) Chmod(name string, mode fs.FileMode) error {
	return os.Chmod(name, mode)
}

// Chown changes the uid and gid of the named file.
// TODO: improve windows support
func (OSFS) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

// Chtimes changes the access and modification times of the named file
func (OSFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}
func (OSFS) Lstat(name string) (fs.FileInfo, error) {
	return os.Lstat(name)
}
func (OSFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}
func (OSFS) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

// TODO: improve windows support
func (OSFS) Lchown(name string, uid int, gid int) error {
	return os.Lchown(name, uid, gid)
}
