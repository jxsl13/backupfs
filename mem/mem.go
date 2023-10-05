package mem

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jxsl13/backupfs/interfaces"
)

const chmodBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky | os.ModeSymlink // Only a subset of bits are allowed to be changed. Documented under os.Chmod()

type MemMapFs struct {
	mu   sync.RWMutex
	data map[string]*FileData
	init sync.Once
}

func NewMemMapFs() interfaces.Fs {
	return &MemMapFs{}
}

func (m *MemMapFs) getData() map[string]*FileData {
	m.init.Do(func() {
		m.data = make(map[string]*FileData)
		// Root should always exist, right?
		// TODO: what about windows?
		root := CreateDir(FilePathSeparator)
		SetMode(root, os.ModeDir|0o755)
		m.data[FilePathSeparator] = root
	})
	return m.data
}

func (*MemMapFs) Name() string { return "MemMapFS" }

func (m *MemMapFs) Create(name string) (interfaces.File, error) {
	return m.create(name)
}

func (m *MemMapFs) create(name string) (*File, error) {
	name = normalizePath(name)
	m.mu.Lock()
	file := CreateFile(name)
	m.getData()[name] = file
	m.registerWithParent(file, 0)
	m.mu.Unlock()
	return NewFileHandle(file), nil
}

func (m *MemMapFs) unRegisterWithParent(fileName string) error {
	f, err := m.lockfreeOpen(fileName)
	if err != nil {
		return err
	}
	parent := m.findParent(f)
	if parent == nil {
		log.Panic("parent of ", f.Name(), " is nil")
	}

	parent.Lock()
	parent.memDir.Remove(f)
	parent.Unlock()
	return nil
}

func (m *MemMapFs) findParent(f *FileData) *FileData {
	pdir, _ := filepath.Split(f.Name())
	pdir = filepath.Clean(pdir)
	pfile, err := m.lockfreeOpen(pdir)
	if err != nil {
		return nil
	}
	return pfile
}

func (m *MemMapFs) findDescendants(name string) []*FileData {
	fData := m.getData()
	descendants := make([]*FileData, 0, len(fData))
	for p, dFile := range fData {
		if strings.HasPrefix(p, name+FilePathSeparator) {
			descendants = append(descendants, dFile)
		}
	}

	sort.Slice(descendants, func(i, j int) bool {
		cur := len(strings.Split(descendants[i].Name(), FilePathSeparator))
		next := len(strings.Split(descendants[j].Name(), FilePathSeparator))
		return cur < next
	})

	return descendants
}

func (m *MemMapFs) registerWithParent(f *FileData, perm fs.FileMode) {
	if f == nil {
		return
	}
	parent := m.findParent(f)
	if parent == nil {
		pdir := filepath.Dir(filepath.Clean(f.Name()))
		err := m.lockfreeMkdir(pdir, perm)
		if err != nil {
			// log.Println("Mkdir error:", err)
			return
		}
		parent, err = m.lockfreeOpen(pdir)
		if err != nil {
			// log.Println("Open after Mkdir error:", err)
			return
		}
	}

	parent.Lock()
	initDirMap(parent)
	parent.memDir.Add(f)
	parent.Unlock()
}

func (m *MemMapFs) lockfreeMkdir(name string, perm fs.FileMode) error {
	name = normalizePath(name)
	x, ok := m.getData()[name]
	if ok {
		// Only return ErrFileExists if it's a interfaces.file, not a directory.
		i := FileInfo{FileData: x}
		if !i.IsDir() {
			return ErrFileExists
		}
	} else {
		item := CreateDir(name)
		SetMode(item, os.ModeDir|perm)
		m.getData()[name] = item
		m.registerWithParent(item, perm)
	}
	return nil
}

func (m *MemMapFs) Mkdir(name string, perm fs.FileMode) error {
	perm &= chmodBits
	name = normalizePath(name)

	m.mu.RLock()
	_, ok := m.getData()[name]
	m.mu.RUnlock()
	if ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: ErrFileExists}
	}

	m.mu.Lock()
	// Dobule check that it doesn't exist.
	if _, ok := m.getData()[name]; ok {
		m.mu.Unlock()
		return &fs.PathError{Op: "mkdir", Path: name, Err: ErrFileExists}
	}
	item := CreateDir(name)
	SetMode(item, os.ModeDir|perm)
	m.getData()[name] = item
	m.registerWithParent(item, perm)
	m.mu.Unlock()

	return m.setFileMode(name, perm|os.ModeDir)
}

func (m *MemMapFs) MkdirAll(path string, perm fs.FileMode) error {
	err := m.Mkdir(path, perm)
	if err != nil {
		if errors.Is(err.(*fs.PathError).Err, ErrFileExists) {
			return nil
		}
		return err
	}
	return nil
}

// Handle some relative paths
func normalizePath(path string) string {
	path = filepath.Clean(path)

	switch path {
	case ".":
		return FilePathSeparator
	case "..":
		return FilePathSeparator
	default:
		return path
	}
}

func (m *MemMapFs) Open(name string) (interfaces.File, error) {
	f, err := m.open(name)
	if f != nil {
		return NewReadOnlyFileHandle(f), err
	}
	return nil, err
}

func (m *MemMapFs) openWrite(name string) (*File, error) {
	f, err := m.open(name)
	if f != nil {
		return NewFileHandle(f), err
	}
	return nil, err
}

func (m *MemMapFs) open(name string) (*FileData, error) {
	name = normalizePath(name)

	m.mu.RLock()
	f, ok := m.getData()[name]
	m.mu.RUnlock()
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: ErrFileNotFound}
	}
	return f, nil
}

func (m *MemMapFs) lockfreeOpen(name string) (*FileData, error) {
	name = normalizePath(name)
	f, ok := m.getData()[name]
	if ok {
		return f, nil
	} else {
		return nil, ErrFileNotFound
	}
}

func (m *MemMapFs) OpenFile(name string, flag int, perm fs.FileMode) (interfaces.File, error) {
	perm &= chmodBits
	chmod := false
	file, err := m.openWrite(name)
	if err == nil && (flag&os.O_EXCL > 0) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: ErrFileExists}
	}
	if errors.Is(err, fs.ErrNotExist) && (flag&os.O_CREATE > 0) {
		file, err = m.create(name)
		chmod = true
	}
	if err != nil {
		return nil, err
	}
	if flag == os.O_RDONLY {
		file = NewReadOnlyFileHandle(file.Data())
	}
	if flag&os.O_APPEND > 0 {
		_, err = file.Seek(0, io.SeekEnd)
		if err != nil {
			file.Close()
			return nil, err
		}
	}
	if flag&os.O_TRUNC > 0 && flag&(os.O_RDWR|os.O_WRONLY) > 0 {
		err = file.Truncate(0)
		if err != nil {
			file.Close()
			return nil, err
		}
	}
	if chmod {
		return file, m.setFileMode(name, perm)
	}
	return file, nil
}

func (m *MemMapFs) Remove(name string) error {
	name = normalizePath(name)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.getData()[name]; ok {
		err := m.unRegisterWithParent(name)
		if err != nil {
			return &fs.PathError{Op: "remove", Path: name, Err: err}
		}
		delete(m.getData(), name)
	} else {
		return &fs.PathError{Op: "remove", Path: name, Err: os.ErrNotExist}
	}
	return nil
}

func (m *MemMapFs) RemoveAll(path string) error {
	path = normalizePath(path)
	m.mu.Lock()
	m.unRegisterWithParent(path)
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for p := range m.getData() {
		if p == path || strings.HasPrefix(p, path+FilePathSeparator) {
			m.mu.RUnlock()
			m.mu.Lock()
			delete(m.getData(), p)
			m.mu.Unlock()
			m.mu.RLock()
		}
	}
	return nil
}

func (m *MemMapFs) Rename(oldname, newname string) error {
	oldname = normalizePath(oldname)
	newname = normalizePath(newname)

	if oldname == newname {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.getData()[oldname]; ok {
		m.mu.RUnlock()
		m.mu.Lock()
		err := m.unRegisterWithParent(oldname)
		if err != nil {
			return err
		}

		fileData := m.getData()[oldname]
		ChangeFileName(fileData, newname)
		m.getData()[newname] = fileData

		err = m.renameDescendants(oldname, newname)
		if err != nil {
			return err
		}

		delete(m.getData(), oldname)

		m.registerWithParent(fileData, 0)
		m.mu.Unlock()
		m.mu.RLock()
	} else {
		return &fs.PathError{Op: "rename", Path: oldname, Err: ErrFileNotFound}
	}
	return nil
}

func (m *MemMapFs) renameDescendants(oldname, newname string) error {
	descendants := m.findDescendants(oldname)
	removes := make([]string, 0, len(descendants))
	for _, desc := range descendants {
		descNewName := strings.Replace(desc.Name(), oldname, newname, 1)
		err := m.unRegisterWithParent(desc.Name())
		if err != nil {
			return err
		}

		removes = append(removes, desc.Name())
		ChangeFileName(desc, descNewName)
		m.getData()[descNewName] = desc

		m.registerWithParent(desc, 0)
	}
	for _, r := range removes {
		delete(m.getData(), r)
	}

	return nil
}

func (m *MemMapFs) Lstat(name string) (os.FileInfo, error) {
	fileInfo, err := m.Stat(name)
	return fileInfo, err
}

func (m *MemMapFs) Stat(name string) (os.FileInfo, error) {
	f, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	fi := GetFileInfo(f.(*File).Data())
	return fi, nil
}

func (m *MemMapFs) Chmod(name string, mode fs.FileMode) error {
	mode &= chmodBits

	m.mu.RLock()
	f, ok := m.getData()[name]
	m.mu.RUnlock()
	if !ok {
		return &fs.PathError{Op: "chmod", Path: name, Err: ErrFileNotFound}
	}
	prevOtherBits := GetFileInfo(f).Mode() & ^chmodBits

	mode = prevOtherBits | mode
	return m.setFileMode(name, mode)
}

func (m *MemMapFs) setFileMode(name string, mode fs.FileMode) error {
	name = normalizePath(name)

	m.mu.RLock()
	f, ok := m.getData()[name]
	m.mu.RUnlock()
	if !ok {
		return &fs.PathError{Op: "chmod", Path: name, Err: ErrFileNotFound}
	}

	m.mu.Lock()
	SetMode(f, mode)
	m.mu.Unlock()

	return nil
}

func (m *MemMapFs) Chown(name string, username, group string) error {
	name = normalizePath(name)

	m.mu.RLock()
	f, ok := m.getData()[name]
	m.mu.RUnlock()
	if !ok {
		return &fs.PathError{Op: "chown", Path: name, Err: ErrFileNotFound}
	}

	SetOwnerUser(f, username)
	SetOwnerGroup(f, group)

	return nil
}

func (m *MemMapFs) Lchown(name string, username, group string) error {
	return m.Chown(name, username, group)
}

func (m *MemMapFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name = normalizePath(name)

	m.mu.RLock()
	f, ok := m.getData()[name]
	m.mu.RUnlock()
	if !ok {
		return &fs.PathError{Op: "chtimes", Path: name, Err: ErrFileNotFound}
	}

	m.mu.Lock()
	SetModTime(f, mtime)
	m.mu.Unlock()

	return nil
}

// Readlink reads the symlink located at name
func (m *MemMapFs) Readlink(name string) (string, error) {
	name = normalizePath(name)
	m.mu.RLock()
	f, ok := m.getData()[name]
	m.mu.RUnlock()
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: ErrFileNotFound}
	}
	if f.mode&os.ModeSymlink == 0 {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: errors.New("not a symlink")}
	}
	return string(f.data), nil
}

func (m *MemMapFs) Symlink(newname, oldname string) error {
	file, err := m.openWrite(newname)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
		}
		file, err = m.create(newname)
		if err != nil {
			return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
		}
		defer file.Close()
		_, err = file.WriteString(oldname)
		if err != nil {
			return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
		}
		return m.setFileMode(newname, os.ModeSymlink|0o777)
	}
	defer file.Close()

	if file.Data().mode&os.ModeSymlink == 0 {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: fmt.Errorf("already exists and is not a symlink")}
	}

	err = file.Truncate(0)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	_, err = file.WriteString(oldname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}
	return nil
}

func (m *MemMapFs) List() {
	for _, x := range m.data {
		y := FileInfo{FileData: x}
		fmt.Println(x.Name(), y.Size())
	}
}
