package osfs

import (
	"io/fs"
	"os"
)

func newOsFile(f *os.File) *osFile {
	return &osFile{
		f: f,
	}
}

type osFile struct {
	f *os.File
}

func (f *osFile) Close() error {
	return f.f.Close()
}
func (f *osFile) Read(p []byte) (n int, err error) {
	return f.f.Read(p)
}
func (f *osFile) ReadAt(p []byte, offset int64) (n int, err error) {
	return f.f.ReadAt(p, offset)
}
func (f *osFile) Seek(offset int64, whence int) (int64, error) {
	return f.f.Seek(offset, whence)
}
func (f *osFile) Write(p []byte) (n int, err error) {
	return f.f.Write(p)
}
func (f *osFile) WriteAt(p []byte, offset int64) (n int, err error) {
	return f.f.WriteAt(p, offset)
}
func (f *osFile) Name() string {
	return f.f.Name()
}
func (f *osFile) Readdir(count int) ([]fs.FileInfo, error) {
	return f.f.Readdir(count)
}
func (f *osFile) Readdirnames(n int) ([]string, error) {
	return f.f.Readdirnames(n)
}
func (f *osFile) Stat() (fs.FileInfo, error) {
	return f.f.Stat()
}
func (f *osFile) Sync() error {
	return f.f.Sync()
}
func (f *osFile) Truncate(size int64) error {
	return f.f.Truncate(size)
}
func (f *osFile) WriteString(s string) (ret int, err error) {
	return f.f.WriteString(s)
}
