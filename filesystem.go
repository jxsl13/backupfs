package backupfs

import (
	"io"
	"io/fs"
	"time"
)

type FS interface {
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

	// Chown changes the uid and gid of the named file.
	Chown(name string, uid, gid int) error

	// Chtimes changes the access and modification times of the named file
	Chtimes(name string, atime time.Time, mtime time.Time) error

	Symlinker
}

type Symlinker interface {
	Lstat(name string) (fs.FileInfo, error)
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	Lchown(name string, uid int, gid int) error
}

// File is implemented by the imported directory.
type File interface {
	fs.File
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
	Read([]byte) (int, error)
	Sync() error
	Truncate(size int64) error
	WriteString(s string) (ret int, err error)
}
