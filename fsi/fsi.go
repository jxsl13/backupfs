package fsi

import (
	"io"
	"io/fs"
	"time"
)

// The Fs interface requires that the underlying interface implements
// the basic filesystem operations as well as Symlink specific operations like
// creation, reading, changing of owner and lstat of symlinks.
type Fs interface {
	// The name of this FileSystem
	Name() string

	// Create creates a file in the filesystem, returning the file and an
	// error, if any happens.
	Create(name string) (File, error)

	// Open opens a file, returning it or an error, if any happens.
	Open(name string) (File, error)

	// OpenFile opens a file using the given flags and the given mode.
	OpenFile(name string, flag int, perm fs.FileMode) (File, error)

	// Mkdir creates a directory in the filesystem, return an error if any
	// happens.
	Mkdir(name string, perm fs.FileMode) error

	// MkdirAll creates a directory path and all parents that does not exist
	// yet.
	MkdirAll(path string, perm fs.FileMode) error

	// Readlink reads the symlink located at name
	Readlink(name string) (string, error)

	// Symlink creates a symlink at newname pointing to oldname
	Symlink(oldname, newname string) error

	// Stat returns a FileInfo describing the named file, or an error, if any
	// happens.
	// Does follow symlinks
	Stat(name string) (fs.FileInfo, error)

	// returns true if Lstat was called
	// Does not follow symlinks
	Lstat(name string) (fs.FileInfo, error)

	// Chmod changes the mode of the named file to mode.
	Chmod(name string, mode fs.FileMode) error

	// Chown changes the uid/owner and gid/group of the named file.
	Chown(name string, uid, gid string) error

	// Own returns the owner username and owner group of a file
	// This operation follows symlinks
	Own(name string) (uid, gid string, err error)

	// Lown returns the owner username and owner group of a file
	// This operation does not follow symlinks
	Lown(name string) (uid, gid string, err error)

	// Lchown changes the owner user and group of the symlink located at
	// name to be username and groupname
	Lchown(name string, uid, gid string) error

	// Remove removes a file identified by name, returning an error, if any
	// happens.
	Remove(name string) error

	// RemoveAll removes a directory path and any children it contains. It
	// does not fail if the path does not exist (return nil).
	RemoveAll(path string) error

	// Rename renames a file.
	Rename(oldname, newname string) error

	// Chtimes changes the access and modification times of the named file
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

type File interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt

	Name() string
	Readdir(count int) ([]fs.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Stat() (fs.FileInfo, error)

	Sync() error
	Truncate(size int64) error
	WriteString(s string) (ret int, err error)

	Own() (uid, gid string, err error)
	Uid() (string, error)
	Gid() (string, error)

	Chown(uid, gid string) error
	Chuid(uid string) error
	Chgid(gid string) error
}
