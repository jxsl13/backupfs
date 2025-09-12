package backupfs

import (
	"io/fs"
)

var _ File = (*prefixFile)(nil)

// filePath and prefix are expected to be normalized (filepath.Clean) paths
func newPrefixFile(f File, openPath, absolutePath string) File {
	// seemingly on Windows File.Name() and FileInfo.Name() behave differently
	return &prefixFile{
		f:            f,
		openPath:     openPath,
		absolutePath: absolutePath,
	}
}

type prefixFile struct {
	f            File
	openPath     string
	absolutePath string
}

func (pf *prefixFile) Name() string {
	// hide the existence of the prefix
	return pf.openPath
}
func (pf *prefixFile) Readdir(count int) ([]fs.FileInfo, error) {
	return pf.f.Readdir(count)
}
func (pf *prefixFile) Readdirnames(n int) ([]string, error) {
	return pf.f.Readdirnames(n)
}
func (pf *prefixFile) Stat() (fs.FileInfo, error) {
	fi, err := pf.f.Stat()
	if err != nil {
		return nil, err
	}
	return newPrefixFileInfo(fi, pf.absolutePath), nil
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
