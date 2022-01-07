package mocks

import (
	os "os"
	time "time"

	"github.com/spf13/afero"
)

// Fs is the filesystem interface.
//
// Any simulated or real filesystem should implement this interface.
type Fs interface {
	// Create creates a file in the filesystem, returning the file and an
	// error, if any happens.
	Create(name string) (afero.File, error)

	// Mkdir creates a directory in the filesystem, return an error if any
	// happens.
	Mkdir(name string, perm os.FileMode) error

	// MkdirAll creates a directory path and all parents that does not exist
	// yet.
	MkdirAll(path string, perm os.FileMode) error

	// Open opens a file, returning it or an error, if any happens.
	Open(name string) (afero.File, error)

	// OpenFile opens a file using the given flags and the given mode.
	OpenFile(name string, flag int, perm os.FileMode) (afero.File, error)

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
	Stat(name string) (os.FileInfo, error)

	// The name of this FileSystem
	Name() string

	// Chmod changes the mode of the named file to mode.
	Chmod(name string, mode os.FileMode) error

	// Chown changes the uid and gid of the named file.
	Chown(name string, uid, gid int) error

	//Chtimes changes the access and modification times of the named file
	Chtimes(name string, atime time.Time, mtime time.Time) error
}
