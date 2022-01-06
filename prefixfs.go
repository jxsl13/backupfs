package backupfs

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

func NewPrefixFs(prefixPath string, fs afero.Fs) *PrefixFs {
	return &PrefixFs{
		prefix: filepath.Clean(prefixPath),
		base:   fs,
	}
}

type PrefixFs struct {
	prefix string
	base   afero.Fs
}

func removeStrings(lookupIn, applyTo string, remove ...string) (cleaned string, found bool) {
	found = false
	for _, rem := range remove {
		i := -1
		for {
			i++
			idx := -1
			switch i % 2 {
			case 0:
				idx = strings.Index(lookupIn, rem)
			case 1:
				idx = strings.LastIndex(lookupIn, rem)
			}

			// as long as we do find rem, try to replace rem
			if idx >= 0 {
				lookupIn = lookupIn[0:idx] + lookupIn[idx+len(rem):]
				applyTo = applyTo[0:idx] + applyTo[idx+len(rem):]
				found = true
				continue
			}

			break
		}

	}
	return applyTo, found
}

var (
	removalSet = []string{"../", "./", "..\\", ".\\"}
)

func (s *PrefixFs) prefixPath(name string) string {
	name = strings.TrimRight(name, "./\\")

	for {
		slashName := filepath.ToSlash(name)
		cleaned, found := removeStrings(slashName, name, removalSet...)
		if found {
			name = cleaned
			continue
		}

		break
	}

	prefixedPath := filepath.Join(s.prefix, filepath.Clean(name))
	return prefixedPath
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *PrefixFs) Create(name string) (File, error) {
	return s.base.Create(s.prefixPath(name))
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *PrefixFs) Mkdir(name string, perm os.FileMode) error {
	return s.base.Mkdir(s.prefixPath(name), perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *PrefixFs) MkdirAll(path string, perm os.FileMode) error {
	return s.base.MkdirAll(s.prefixPath(path), perm)
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *PrefixFs) Open(name string) (File, error) {
	return s.base.Open(s.prefixPath(name))
}

// OpenFile opens a file using the given flags and the given mode.
func (s *PrefixFs) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return s.base.OpenFile(s.prefixPath(name), flag, perm)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *PrefixFs) Remove(name string) error {
	return s.base.Remove(s.prefixPath(name))
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *PrefixFs) RemoveAll(path string) error {
	return s.base.RemoveAll(s.prefixPath(path))
}

// Rename renames a file.
func (s *PrefixFs) Rename(oldname, newname string) error {
	return s.base.Rename(s.prefixPath(oldname), s.prefixPath(newname))
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *PrefixFs) Stat(name string) (os.FileInfo, error) {
	return s.base.Stat(s.prefixPath(name))
}

// The name of this FileSystem
func (s *PrefixFs) Name() string {
	return "PrefixFs"
}

// Chmod changes the mode of the named file to mode.
func (s *PrefixFs) Chmod(name string, mode os.FileMode) error {
	return s.base.Chmod(s.prefixPath(name), mode)
}

// Chown changes the uid and gid of the named file.
func (s *PrefixFs) Chown(name string, uid, gid int) error {
	return s.base.Chown(s.prefixPath(name), uid, gid)
}

//Chtimes changes the access and modification times of the named file
func (s *PrefixFs) Chtimes(name string, atime, mtime time.Time) error {
	return s.base.Chtimes(s.prefixPath(name), atime, mtime)
}

// LstatIfPossible will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *PrefixFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	name = s.prefixPath(name)

	if l, ok := s.base.(afero.Lstater); ok {
		// implements interface
		return l.LstatIfPossible(name)
	}

	// does not implement lstat, fallback to stat
	fi, err := s.base.Stat(name)
	return fi, false, err

}
