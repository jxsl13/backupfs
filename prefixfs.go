package backupfs

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// assert interfaces implemented
var (
	_ FS = (*PrefixFS)(nil)
)

// NewPrefixFS creates a new file system abstraction that forces any path to be prepended with
// the provided prefix.
// the existence of the prefixPath existing is hidden away (errors might show full paths).
func NewPrefixFS(prefixPath string, fs FS) *PrefixFS {
	return &PrefixFS{
		prefix: filepath.Clean(prefixPath),
		base:   fs,
	}
}

// PrefixFS, contrary to BasePathFs, does abstract away the existence of a base path.
// The prefixed path is seen as the root directory.
type PrefixFS struct {
	prefix string
	base   FS
}

func (s *PrefixFS) prefixPath(name string) (string, error) {
	volume := filepath.VolumeName(name)

	if volume != "" {
		// interestind for windows, as this backup mechanism does not exactly work
		// with prefixed directories otherwise. A colon is not allowed inisde of the file path.
		// prefix path with volume letter but without the :
		volumeName := strings.TrimRight(volume, ":\\/")
		nameWithoutVolume := strings.TrimLeft(name, volume)
		name = filepath.Join(volumeName, nameWithoutVolume)
	}

	p := filepath.Join(s.prefix, filepath.Clean(name))
	if !strings.HasPrefix(p, s.prefix) {
		return "", fs.ErrNotExist
	}
	return p, nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *PrefixFS) Create(name string) (File, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, err
	}
	f, err := s.base.Create(path)
	if err != nil {
		return nil, err
	}

	return &PrefixFile{f: f, prefix: s.prefix}, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *PrefixFS) Mkdir(name string, perm fs.FileMode) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}
	return s.base.Mkdir(path, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *PrefixFS) MkdirAll(name string, perm fs.FileMode) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}

	return s.base.MkdirAll(path, perm)
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *PrefixFS) Open(name string) (File, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, err
	}

	f, err := s.base.Open(path)
	if err != nil {
		return nil, err
	}

	return &PrefixFile{f: f, prefix: s.prefix}, nil
}

// OpenFile opens a file using the given flags and the given mode.
func (s *PrefixFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, err
	}

	f, err := s.base.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

	return &PrefixFile{f: f, prefix: s.prefix}, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *PrefixFS) Remove(name string) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}

	return s.base.Remove(path)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *PrefixFS) RemoveAll(name string) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}
	return s.base.RemoveAll(path)
}

// Rename renames a file.
func (s *PrefixFS) Rename(oldname, newname string) error {
	oldpath, err := s.prefixPath(oldname)
	if err != nil {
		return err
	}

	newpath, err := s.prefixPath(newname)
	if err != nil {
		return syscall.EPERM
	}
	return s.base.Rename(oldpath, newpath)
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *PrefixFS) Stat(name string) (fs.FileInfo, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, err
	}

	fi, err := s.base.Stat(path)
	if err != nil {
		return nil, err
	}

	return newPrefixFileInfo(fi, s.prefix), nil
}

// The name of this FileSystem
func (s *PrefixFS) Name() string {
	return "PrefixFS"
}

// Chmod changes the mode of the named file to mode.
func (s *PrefixFS) Chmod(name string, mode fs.FileMode) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}

	return s.base.Chmod(path, mode)
}

// Chown changes the uid and gid of the named file.
func (s *PrefixFS) Chown(name string, uid, gid int) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}

	return s.base.Chown(path, uid, gid)
}

// Chtimes changes the access and modification times of the named file
func (s *PrefixFS) Chtimes(name string, atime, mtime time.Time) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}
	return s.base.Chtimes(path, atime, mtime)
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *PrefixFS) Lstat(name string) (fs.FileInfo, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, err
	}

	fi, err := s.base.Lstat(path)
	if err != nil {
		return nil, err
	}
	return newPrefixFileInfo(fi, s.prefix), nil
}

// Symlink changes the access and modification times of the named file
func (s *PrefixFS) Symlink(oldname, newname string) error {
	// links may be relative paths

	var (
		err     error
		oldPath string
	)
	if path.IsAbs(filepath.ToSlash(oldname)) || filepath.IsAbs(filepath.FromSlash(oldname)) {
		// absolute path symlink
		oldPath, err = s.prefixPath(oldname)
	} else {
		// relative path symlink
		_, err = s.prefixPath(filepath.Join(filepath.Dir(newname), oldname))
		oldPath = oldname
	}

	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	newPath, err := s.prefixPath(newname)
	if err != nil {
		return err
	}

	return s.base.Symlink(oldPath, newPath)
}

func (s *PrefixFS) Readlink(name string) (string, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return "", err
	}

	linkedPath, err := s.base.Readlink(path)
	if err != nil {
		return "", err
	}
	cleanedPath := filepath.Clean(linkedPath)

	prefixlessPath := strings.TrimPrefix(cleanedPath, s.prefix)
	return prefixlessPath, nil
}

func (s *PrefixFS) Lchown(name string, uid, gid int) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return err
	}

	return s.base.Lchown(path, uid, gid)
}
