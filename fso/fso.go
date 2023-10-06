package fso

import (
	"io/fs"
	"os"
	"time"

	"github.com/jxsl13/backupfs/fsi"
)

var (
	_ fsi.Fs = (*OsFs)(nil)
)

func New() *OsFs {
	return &OsFs{}
}

type OsFs struct{}

func (*OsFs) Name() string { return "OsFs" }

func (ofs *OsFs) Stat(name string) (os.FileInfo, error) {
	return ofs.stat(name)
}

func (ofs *OsFs) stat(name string) (os.FileInfo, error) {
	_, fi, err := ofs.followSymlinks(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	return fi, nil
}

func (ofs *OsFs) Lstat(name string) (os.FileInfo, error) {
	return ofs.lstat(name)
}

func (ofs *OsFs) lstat(name string) (fs.FileInfo, error) {
	return os.Lstat(name)
}

func (ofs *OsFs) Create(name string) (fsi.File, error) {
	return ofs.create(name)
}

func (ofs *OsFs) create(name string) (fsi.File, error) {
	return ofs.openFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (ofs *OsFs) Open(name string) (fsi.File, error) {
	return ofs.open(name)
}

func (ofs *OsFs) open(name string) (fsi.File, error) {
	return ofs.openFile(name, os.O_RDONLY, 0)
}

func (ofs *OsFs) OpenFile(name string, flag int, perm fs.FileMode) (fsi.File, error) {
	return ofs.openFile(name, flag, perm)
}

func (ofs *OsFs) Remove(name string) error {
	return os.Remove(name)
}

func (ofs *OsFs) remove(name string) error {
	return ofs.remove(name)
}

func (ofs *OsFs) RemoveAll(path string) error {
	return ofs.removeAll(path)
}

func (ofs *OsFs) removeAll(path string) error {
	return os.RemoveAll(path)
}

func (ofs *OsFs) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

func (ofs *OsFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return ofs.chtimes(name, atime, mtime)
}

func (ofs *OsFs) chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

func (ofs *OsFs) Symlink(oldname, newname string) error {
	return ofs.symlink(oldname, newname)
}

func (ofs *OsFs) symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (ofs *OsFs) Readlink(name string) (string, error) {
	return ofs.readlink(name)
}

func (ofs *OsFs) readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (ofs *OsFs) Chmod(name string, mode fs.FileMode) error {
	return ofs.chmod(name, mode)
}

func (ofs *OsFs) Own(name string) (uid, gid string, err error) {
	return ofs.own(name)
}

func (ofs *OsFs) Chown(name string, uid, gid string) error {
	return ofs.chown(name, uid, gid)
}

func (ofs *OsFs) Lchown(name string, uid, gid string) error {
	return ofs.lchown(name, uid, gid)
}

func (ofs *OsFs) Lown(name string) (uid, gid string, err error) {
	return ofs.lown(name)
}

func (ofs *OsFs) Mkdir(name string, perm fs.FileMode) (err error) {
	return ofs.mkdir(name, perm)
}

func (ofs *OsFs) MkdirAll(name string, perm fs.FileMode) (err error) {
	return ofs.mkdirAll(name, perm)
}
