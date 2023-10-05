package interfaces

import (
	"errors"
	"io"
	"io/fs"
	"time"
)

var (
	// ErrNoLchown is the error that will be wrapped in an os.Path if a file system
	// does not support the lchown operation either directly or through its delegated filesystem.
	// As expressed by support for the LinkOwner interface.
	ErrNoLchown = errors.New("lchown not supported")

	// ErrNoLstat can be returned in case that Lstat is not supported on your filesystem
	ErrNoLstat = errors.New("lstat not supported")
)

// The Fs interface requires that the underlying interface implements
// the basic filesystem operations as well as Symlink specific operations like
// creation, reading, changing of owner and lstat of symlinks.
type Fs interface {
	// Create creates a file in the filesystem, returning the file and an
	// error, if any happens.
	Create(name string) (File, error)

	// Mkdir creates a directory in the filesystem, return an error if any
	// happens.
	Mkdir(name string, perm fs.FileMode) error

	// MkdirAll creates a directory path and all parents that does not exist
	// yet.
	MkdirAll(path string, perm fs.FileMode) error

	// Open opens a file, returning it or an error, if any happens.
	Open(name string) (File, error)

	// OpenFile opens a file using the given flags and the given mode.
	OpenFile(name string, flag int, perm fs.FileMode) (File, error)

	// Remove removes a file identified by name, returning an error, if any
	// happens.
	Remove(name string) error

	// RemoveAll removes a directory path and any children it contains. It
	// does not fail if the path does not exist (return nil).
	RemoveAll(path string) error

	// Rename renames a file.
	Rename(oldname, newname string) error

	// Stat returns a FileInfo describing the named file, or an error, if any
	// happens.
	Stat(name string) (fs.FileInfo, error)

	// The name of this FileSystem
	Name() string

	// Chmod changes the mode of the named file to mode.
	Chmod(name string, mode fs.FileMode) error

	// Chown changes the uid/owner and gid/group of the named file.
	Chown(name string, username, groupname string) error

	// Chtimes changes the access and modification times of the named file
	Chtimes(name string, atime time.Time, mtime time.Time) error

	// returns true if Lstat was called
	Lstat(name string) (fs.FileInfo, error)

	// Symlink creates a symlink at newname pointing to oldname
	Symlink(oldname, newname string) error

	// Readlink reads the symlink located at name
	Readlink(name string) (string, error)

	// Lchown changes the owner user and group of the symlink located at
	// name to be username and groupname
	Lchown(name string, username, groupname string) error
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

	OwnerUser() (string, error)
	OwnerGroup() (string, error)
	OwnerUid() (string, error)
	OwnerGid() (string, error)

	SetOwnerUser(user string) error
	SetOwnerGroup(group string) error
	SetOwnerUid(uid string) error
	SetOwnerGid(gid string) error
}
