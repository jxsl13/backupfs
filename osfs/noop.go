package osfs

import (
	"io/fs"
	"time"

	"github.com/jxsl13/backupfs/interfaces"
)

var (
	_ interfaces.Fs = (*NoOpFs)(nil)
)

func NewNoOpFs() interfaces.Fs {
	return &NoOpFs{}
}

// NoOpFs does nothing
type NoOpFs struct{}

func (NoOpFs) Name() string { return "NoOpFs" }

func (NoOpFs) Create(name string) (interfaces.File, error) {
	return nil, notImplemented("create", name)
}

func (NoOpFs) Mkdir(name string, perm fs.FileMode) error {
	return notImplemented("mkdir", name)
}

func (NoOpFs) MkdirAll(path string, perm fs.FileMode) error {
	return notImplemented("mkdir_all", path)
}

func (NoOpFs) Open(name string) (interfaces.File, error) {
	return nil, notImplemented("open", name)
}

func (NoOpFs) OpenFile(name string, flag int, perm fs.FileMode) (interfaces.File, error) {
	return nil, notImplemented("open_file", name)
}

func (NoOpFs) Remove(name string) error {
	return notImplemented("remove", name)
}

func (NoOpFs) RemoveAll(path string) error {
	return notImplemented("remove_all", path)
}

func (NoOpFs) Rename(oldname, newname string) error {
	return notImplemented("rename", oldname)
}

func (NoOpFs) Stat(name string) (fs.FileInfo, error) {
	return nil, notImplemented("stat", name)
}

func (NoOpFs) Lstat(name string) (fs.FileInfo, error) {
	return nil, notImplemented("lstat", name)
}

func (NoOpFs) Chmod(name string, mode fs.FileMode) error {
	return notImplemented("chmod", name)
}

func (NoOpFs) Chown(name string, username, group string) error {
	return notImplemented("chown", name)
}

func (NoOpFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return notImplemented("chtimes", name)
}

func (NoOpFs) LstatIfPossible(name string) (fs.FileInfo, bool, error) {
	return nil, false, notImplemented("lstat", name)
}

func (NoOpFs) Symlink(oldname, newname string) error {
	return notImplemented("symlink", oldname)
}

func (NoOpFs) Readlink(name string) (string, error) {
	return "", notImplemented("readlink", name)
}

func (NoOpFs) Lchown(name string, username, group string) error {
	return notImplemented("lchown", name)
}

func notImplemented(op, path string) error {
	return &fs.PathError{Op: op, Path: path, Err: fs.ErrPermission}
}
