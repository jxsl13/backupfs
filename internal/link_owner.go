package internal

import "errors"

// LinkOwner is an optional interface in Afero. It is only implemented by the
// filesystems saying so.
type LinkOwner interface {
	LchownIfPossible(name string, uid int, gid int) error
}

// ErrNoLchown is the error that will be wrapped in an os.Path if a file system
// does not support the lchown operation either directly or through its delegated filesystem.
// As expressed by support for the LinkOwner interface.
var ErrNoLchown = errors.New("lchown not supported")
