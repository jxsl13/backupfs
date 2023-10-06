package mem

import (
	"os"
	"path/filepath"
	"time"
)

type FileInfo struct {
	*FileData
}

// Implements os.FileInfo
func (s *FileInfo) Name() string {
	s.mu.Lock()
	_, name := filepath.Split(s.name)
	s.mu.Unlock()
	return name
}

func (s *FileInfo) Mode() os.FileMode {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.mode
}

func (s *FileInfo) ModTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.modtime
}

func (s *FileInfo) IsDir() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dir
}
func (s *FileInfo) Sys() interface{} { return nil }
func (s *FileInfo) Size() int64 {
	if s.IsDir() {
		return int64(42)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.data))
}
