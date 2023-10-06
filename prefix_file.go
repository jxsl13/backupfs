package backupfs

import (
	"io/fs"
	"strings"

	"github.com/jxsl13/backupfs/fsi"
)

var _ fsi.File = (*PrefixFile)(nil)

type PrefixFile struct {
	f fsi.File
	// this prefix is clean due to th eFs prefix being clean
	prefix string
}

func (pf *PrefixFile) Name() string {
	// hide the existence of the prefix
	return strings.TrimPrefix(pf.f.Name(), pf.prefix)
}
func (pf *PrefixFile) Readdir(count int) ([]fs.FileInfo, error) {
	return pf.f.Readdir(count)
}
func (pf *PrefixFile) Readdirnames(n int) ([]string, error) {
	return pf.f.Readdirnames(n)
}
func (pf *PrefixFile) Stat() (fs.FileInfo, error) {
	return pf.f.Stat()
}
func (pf *PrefixFile) Sync() error {
	return pf.f.Sync()
}
func (pf *PrefixFile) Truncate(size int64) error {
	return pf.f.Truncate(size)
}
func (pf *PrefixFile) WriteString(s string) (ret int, err error) {
	return pf.f.WriteString(s)
}

func (pf *PrefixFile) Close() error {
	return pf.f.Close()
}

func (pf *PrefixFile) Read(p []byte) (n int, err error) {
	return pf.f.Read(p)
}

func (pf *PrefixFile) ReadAt(p []byte, off int64) (n int, err error) {
	return pf.f.ReadAt(p, off)
}

func (pf *PrefixFile) Seek(offset int64, whence int) (int64, error) {
	return pf.f.Seek(offset, whence)
}

func (pf *PrefixFile) Write(p []byte) (n int, err error) {
	return pf.f.Write(p)
}

func (pf *PrefixFile) WriteAt(p []byte, off int64) (n int, err error) {
	return pf.f.WriteAt(p, off)
}

func (pf *PrefixFile) SetOwnerUser(username string) (err error) {
	return pf.f.SetOwnerUser(username)
}

func (pf *PrefixFile) SetOwnerGroup(group string) (err error) {
	return pf.f.SetOwnerGroup(group)
}

func (pf *PrefixFile) SetOwnerUid(uid string) (err error) {
	return pf.f.SetOwnerUid(uid)
}

func (pf *PrefixFile) SetOwnerGid(gid string) (err error) {
	return pf.f.SetOwnerGid(gid)
}

func (pf *PrefixFile) OwnerUser() (username string, err error) {
	return pf.f.OwnerUser()
}

func (pf *PrefixFile) OwnerGroup() (group string, err error) {
	return pf.f.OwnerGroup()
}

func (pf *PrefixFile) OwnerUid() (uid string, err error) {
	return pf.f.OwnerUid()
}

func (pf *PrefixFile) OwnerGid() (gid string, err error) {
	return pf.f.OwnerGid()
}
