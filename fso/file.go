package fso

import (
	"io/fs"
	"os"
	"sync"
)

func newOsFile(fs *OsFs, f *os.File) *osFile {
	return &osFile{
		fs: fs,
		f:  f,
	}
}

type osFile struct {
	fs *OsFs
	// only lock for file operations
	mu sync.Mutex
	f  *os.File
}

func (f *osFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.f.Close()
}
func (f *osFile) Read(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Read(p)
}
func (f *osFile) ReadAt(p []byte, offset int64) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.ReadAt(p, offset)
}
func (f *osFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Seek(offset, whence)
}
func (f *osFile) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Write(p)
}
func (f *osFile) WriteAt(p []byte, offset int64) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.WriteAt(p, offset)
}
func (f *osFile) Name() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.name()
}

func (f *osFile) name() string {
	return f.f.Name()
}

func (f *osFile) Readdir(count int) ([]fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Readdir(count)
}

func (f *osFile) Readdirnames(n int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Readdirnames(n)
}

func (f *osFile) Stat() (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Stat()
}

func (f *osFile) Sync() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Sync()
}

func (f *osFile) Truncate(size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.Truncate(size)
}

func (f *osFile) WriteString(s string) (ret int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.f.WriteString(s)
}

func (f *osFile) Uid() (uid string, err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "uid", Path: f.name(), Err: err}
		}
	}()

	return f.uid()
}

func (f *osFile) Gid() (gid string, err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "uid", Path: f.name(), Err: err}
		}
	}()

	return f.gid()
}

func (f *osFile) Own() (uid, gid string, err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "own", Path: f.name(), Err: err}
		}
	}()

	return f.own()
}

func (f *osFile) Chown(uid, gid string) (err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "chown", Path: f.name(), Err: err}
		}
	}()

	return f.chown(uid, gid)
}

func (f *osFile) Chuid(uid string) (err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "chuid", Path: f.name(), Err: err}
		}
	}()

	return f.chuid(uid)
}

func (f *osFile) Chgid(gid string) (err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "chgid", Path: f.name(), Err: err}
		}
	}()

	return f.chgid(gid)
}
