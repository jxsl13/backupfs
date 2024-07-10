package backupfs

import (
	"io/fs"
	"strings"
)

var _ File = (*prefixFile)(nil)

type prefixFile struct {
	f File
	// this prefix is clean due to th eFS prefix being clean
	prefix string
}

func (pf *prefixFile) Name() string {
	// hide the existence of the prefix
	return strings.TrimPrefix(pf.f.Name(), pf.prefix)
}
func (pf *prefixFile) Readdir(count int) ([]fs.FileInfo, error) {
	return pf.f.Readdir(count)
}
func (pf *prefixFile) Readdirnames(n int) ([]string, error) {
	return pf.f.Readdirnames(n)
}
func (pf *prefixFile) Stat() (fs.FileInfo, error) {
	return pf.f.Stat()
}
func (pf *prefixFile) Sync() error {
	return pf.f.Sync()
}
func (pf *prefixFile) Truncate(size int64) error {
	return pf.f.Truncate(size)
}
func (pf *prefixFile) WriteString(s string) (ret int, err error) {
	return pf.f.WriteString(s)
}

func (pf *prefixFile) Close() error {
	err := pf.f.Close()
	if err != nil {
		return err
	}
	return nil
}

func (pf *prefixFile) Read(p []byte) (n int, err error) {
	return pf.f.Read(p)
}

func (pf *prefixFile) ReadAt(p []byte, off int64) (n int, err error) {
	return pf.f.ReadAt(p, off)
}

func (pf *prefixFile) Seek(offset int64, whence int) (int64, error) {
	return pf.f.Seek(offset, whence)
}

func (pf *prefixFile) Write(p []byte) (n int, err error) {
	return pf.f.Write(p)
}

func (pf *prefixFile) WriteAt(p []byte, off int64) (n int, err error) {
	return pf.f.WriteAt(p, off)
}
