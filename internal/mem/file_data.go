package mem

import (
	"io/fs"
	"os"
	"sync"
	"time"
)

func CreateFile(name string, perm fs.FileMode) *FileData {
	fd := &FileData{name: name, mode: os.ModeTemporary, modtime: time.Now()}
	fd.SetMode(perm.Perm())
	return fd
}

func CreateDir(name string, perm fs.FileMode) *FileData {
	fd := &FileData{name: name, memDir: &DirMap{}, dir: true, modtime: time.Now()}
	fd.SetMode(fs.ModeDir | perm.Perm())
	return fd
}

type FileData struct {
	mu      sync.Mutex
	name    string
	data    []byte
	memDir  Dir
	dir     bool
	mode    fs.FileMode
	modtime time.Time
	uid     string
	gid     string
}

func (d *FileData) Bytes() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]byte, len(d.data))
	copy(result, d.data)
	return result
}

func (d *FileData) Uid() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.uid
}

func (d *FileData) Gid() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.gid
}

func (d *FileData) Own() (uid, gid string, _ error) {
	return d.uid, d.gid, nil
}

func (d *FileData) Chown(uid, gid string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.uid = uid
	d.gid = gid
	return nil
}

func (d *FileData) Chuid(uid string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.uid = uid
	return nil
}

func (d *FileData) Chgid(gid string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gid = gid
	return nil
}

func (d *FileData) Name() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.name
}

func (d *FileData) Rename(newname string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.name = newname
}

func (d *FileData) SetMode(mode fs.FileMode) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mode = mode
}

func (d *FileData) SetModTime(mtime time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.setModTime(mtime)
}

func (d *FileData) setModTime(mtime time.Time) {
	d.modtime = mtime
}

func (d *FileData) FileInfo() *FileInfo {
	return &FileInfo{d}
}
