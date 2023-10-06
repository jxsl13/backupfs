package mem

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/jxsl13/backupfs/fsi"
)

const FilePathSeparator = string(filepath.Separator)

var (
	_ fsi.File       = (*File)(nil)
	_ fs.ReadDirFile = (*File)(nil)
)

type File struct {
	// atomic requires 64-bit alignment for struct field access
	at           int64
	readDirCount int64
	closed       bool
	readOnly     bool
	fileData     *FileData
}

func NewFileHandle(data *FileData) *File {
	return &File{fileData: data}
}

func NewReadOnlyFileHandle(data *FileData) *File {
	return &File{fileData: data, readOnly: true}
}

func (f File) Data() *FileData {
	return f.fileData
}

func (f *File) Open() error {
	atomic.StoreInt64(&f.at, 0)
	atomic.StoreInt64(&f.readDirCount, 0)
	f.fileData.mu.Lock()
	f.closed = false
	f.fileData.mu.Unlock()
	return nil
}

func (f *File) Close() error {
	f.fileData.mu.Lock()
	f.closed = true
	if !f.readOnly {
		f.fileData.setModTime(time.Now())
	}
	f.fileData.mu.Unlock()
	return nil
}

func (f *File) Name() string {
	return f.fileData.Name()
}

func (f *File) Stat() (os.FileInfo, error) {
	return &FileInfo{f.fileData}, nil
}

func (f *File) Sync() error {
	return nil
}

func (f *File) Readdir(count int) (res []os.FileInfo, err error) {
	if !f.fileData.dir {
		return nil, &os.PathError{Op: "readdir", Path: f.fileData.name, Err: errors.New("not a dir")}
	}
	var outLength int64

	f.fileData.mu.Lock()
	files := f.fileData.memDir.Files()[f.readDirCount:]
	if count > 0 {
		if len(files) < count {
			outLength = int64(len(files))
		} else {
			outLength = int64(count)
		}
		if len(files) == 0 {
			err = io.EOF
		}
	} else {
		outLength = int64(len(files))
	}
	f.readDirCount += outLength
	f.fileData.mu.Unlock()

	res = make([]os.FileInfo, outLength)
	for i := range res {
		res[i] = &FileInfo{files[i]}
	}

	return res, err
}

func (f *File) Readdirnames(n int) (names []string, err error) {
	fi, err := f.Readdir(n)
	names = make([]string, len(fi))
	for i, f := range fi {
		_, names[i] = filepath.Split(f.Name())
	}
	return names, err
}

// Implements fs.ReadDirFile
func (f *File) ReadDir(n int) ([]fs.DirEntry, error) {
	fi, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}
	di := make([]fs.DirEntry, len(fi))
	for i, f := range fi {
		di[i] = FileInfoDirEntry{FileInfo: f}
	}
	return di, nil
}

func (f *File) Read(b []byte) (n int, err error) {
	f.fileData.mu.Lock()
	defer f.fileData.mu.Unlock()
	if f.closed {
		return 0, ErrFileClosed
	}
	if len(b) > 0 && int(f.at) == len(f.fileData.data) {
		return 0, io.EOF
	}
	if int(f.at) > len(f.fileData.data) {
		return 0, io.ErrUnexpectedEOF
	}
	if len(f.fileData.data)-int(f.at) >= len(b) {
		n = len(b)
	} else {
		n = len(f.fileData.data) - int(f.at)
	}
	copy(b, f.fileData.data[f.at:f.at+int64(n)])
	atomic.AddInt64(&f.at, int64(n))
	return
}

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	prev := atomic.LoadInt64(&f.at)
	atomic.StoreInt64(&f.at, off)
	n, err = f.Read(b)
	atomic.StoreInt64(&f.at, prev)
	return
}

func (f *File) Truncate(size int64) error {
	if f.closed {
		return ErrFileClosed
	}
	if f.readOnly {
		return &os.PathError{Op: "truncate", Path: f.fileData.name, Err: errors.New("file handle is read only")}
	}
	if size < 0 {
		return ErrOutOfRange
	}
	f.fileData.mu.Lock()
	defer f.fileData.mu.Unlock()
	if size > int64(len(f.fileData.data)) {
		diff := size - int64(len(f.fileData.data))
		f.fileData.data = append(f.fileData.data, bytes.Repeat([]byte{0x0}, int(diff))...)
	} else {
		f.fileData.data = f.fileData.data[0:size]
	}
	f.fileData.setModTime(time.Now())
	return nil
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, ErrFileClosed
	}
	switch whence {
	case io.SeekStart:
		atomic.StoreInt64(&f.at, offset)
	case io.SeekCurrent:
		atomic.AddInt64(&f.at, offset)
	case io.SeekEnd:
		atomic.StoreInt64(&f.at, int64(len(f.fileData.data))+offset)
	}
	return f.at, nil
}

func (f *File) Write(b []byte) (n int, err error) {
	if f.closed {
		return 0, ErrFileClosed
	}
	if f.readOnly {
		return 0, &os.PathError{Op: "write", Path: f.fileData.name, Err: errors.New("file handle is read only")}
	}
	n = len(b)
	cur := atomic.LoadInt64(&f.at)
	f.fileData.mu.Lock()
	defer f.fileData.mu.Unlock()
	diff := cur - int64(len(f.fileData.data))
	var tail []byte
	if n+int(cur) < len(f.fileData.data) {
		tail = f.fileData.data[n+int(cur):]
	}
	if diff > 0 {
		f.fileData.data = append(f.fileData.data, append(bytes.Repeat([]byte{0x0}, int(diff)), b...)...)
		f.fileData.data = append(f.fileData.data, tail...)
	} else {
		f.fileData.data = append(f.fileData.data[:cur], b...)
		f.fileData.data = append(f.fileData.data, tail...)
	}
	f.fileData.setModTime(time.Now())

	atomic.AddInt64(&f.at, int64(n))
	return
}

func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	atomic.StoreInt64(&f.at, off)
	return f.Write(b)
}

func (f *File) WriteString(s string) (ret int, err error) {
	return f.Write([]byte(s))
}

func (f *File) Info() *FileInfo {
	return &FileInfo{f.fileData}
}

func (f *File) Own() (uid, gid string, err error) {
	return f.Data().Own()
}

func (f *File) Gid() (string, error) {
	return f.Data().Gid(), nil
}

func (f *File) Uid() (string, error) {
	return f.Data().Uid(), nil
}

func (f *File) Chown(uid, gid string) error {
	return f.Data().Chown(uid, gid)
}

func (f *File) Chuid(uid string) error {
	return f.Data().Chuid(uid)
}

func (f *File) Chgid(gid string) error {
	return f.Data().Chgid(gid)
}
