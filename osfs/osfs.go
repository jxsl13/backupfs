package osfs

import (
	"io/fs"
	"os"
	"time"

	"github.com/jxsl13/backupfs/interfaces"
)

var (
	_ interfaces.Fs = (*OsFs)(nil)
)

func New() *OsFs {
	return &OsFs{}
}

type OsFs struct{}

func (OsFs) Name() string { return "OsFs" }

func (ofs OsFs) Create(name string) (interfaces.File, error) {
	return ofs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (ofs OsFs) Open(name string) (interfaces.File, error) {
	return ofs.OpenFile(name, os.O_RDONLY, 0)
}

func (OsFs) OpenFile(name string, flag int, perm fs.FileMode) (interfaces.File, error) {
	f, e := os.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return newOsFile(f), e
}

func (OsFs) Remove(name string) error {
	return os.Remove(name)
}

func (OsFs) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (OsFs) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

func (OsFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

func (OsFs) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (OsFs) Readlink(name string) (string, error) {
	return os.Readlink(name)
}
