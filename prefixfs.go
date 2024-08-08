package backupfs

import (
	"io/fs"
	"os"
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
// The prefixPath is seen as the root directory.
func NewPrefixFS(fs FS, prefixPath string) *PrefixFS {
	return &PrefixFS{
		prefix: filepath.Clean(prefixPath),
		base:   fs,
	}
}

// PrefixFS, contrary to BasePathFS, does abstract away the existence of a base path.
// The prefixed path is seen as the root directory.
type PrefixFS struct {
	prefix string
	base   FS
}

func (s *PrefixFS) prefixPath(name string) (string, error) {
	volume := filepath.VolumeName(name)

	if volume != "" {
		// interesting for windows, as this backup mechanism does not exactly work
		// with prefixed directories otherwise. A colon is not allowed inisde of the file path.
		// prefix path with volume letter but without the :
		volumeName := strings.TrimRight(volume, ":\\/")
		nameWithoutVolume := strings.TrimLeft(name, volume)
		name = filepath.Join(volumeName, nameWithoutVolume)
	}

	p := filepath.Join(s.prefix, filepath.Clean(name))
	if !strings.HasPrefix(p, s.prefix) {
		return "", syscall.EPERM
	}
	return p, nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *PrefixFS) Create(name string) (File, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: err}
	}
	f, err := s.base.Create(path)
	if err != nil {
		return nil, err
	}

	return newPrefixFile(f, path, s.prefix), nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *PrefixFS) Mkdir(name string, perm fs.FileMode) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}
	err = s.base.Mkdir(path, perm)
	if err != nil {
		return err
	}
	return nil
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *PrefixFS) MkdirAll(name string, perm fs.FileMode) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir_all", Path: name, Err: err}
	}

	err = s.base.MkdirAll(path, perm)
	if err != nil {
		return err
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *PrefixFS) Open(name string) (File, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	f, err := s.base.Open(path)
	if err != nil {
		return nil, err
	}

	return newPrefixFile(f, path, s.prefix), nil
}

// OpenFile opens a file using the given flags and the given mode.
func (s *PrefixFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open_file", Path: name, Err: err}
	}

	f, err := s.base.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

	return newPrefixFile(f, path, s.prefix), nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *PrefixFS) Remove(name string) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	err = s.base.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *PrefixFS) RemoveAll(name string) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "remove_all", Path: name, Err: err}
	}
	err = s.base.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}

// Rename renames a file.
func (s *PrefixFS) Rename(oldname, newname string) error {
	oldpath, err := s.prefixPath(oldname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: oldname, Err: err}
	}

	newpath, err := s.prefixPath(newname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: err}
	}
	err = s.base.Rename(oldpath, newpath)
	if err != nil {
		return err
	}
	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *PrefixFS) Stat(name string) (fs.FileInfo, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	fi, err := s.base.Stat(path)
	if err != nil {
		return nil, err
	}

	return newPrefixFileInfo(fi, path, s.prefix), nil
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

	err = s.base.Chmod(path, mode)
	if err != nil {
		return err
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (s *PrefixFS) Chown(name string, uid, gid int) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chown", Path: name, Err: err}
	}

	err = s.base.Chown(path, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (s *PrefixFS) Chtimes(name string, atime, mtime time.Time) error {
	path, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chtimes", Path: name, Err: err}
	}
	err = s.base.Chtimes(path, atime, mtime)
	if err != nil {
		return err
	}
	return nil
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *PrefixFS) Lstat(name string) (fs.FileInfo, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}

	fi, err := s.base.Lstat(path)
	if err != nil {
		return nil, err
	}
	return newPrefixFileInfo(fi, path, s.prefix), nil
}

// Symlink changes the access and modification times of the named file
func (s *PrefixFS) Symlink(oldname, newname string) error {
	// links may be relative paths

	var (
		err     error
		oldPath string
	)
	if isAbs(oldname) {
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
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	err = s.base.Symlink(oldPath, newPath)
	if err != nil {
		return err
	}
	return nil
}

func (s *PrefixFS) Readlink(name string) (string, error) {
	path, err := s.prefixPath(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
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
		return &fs.PathError{Op: "lchown", Path: name, Err: err}
	}

	err = s.base.Lchown(path, uid, gid)
	if err != nil {
		return err
	}
	return nil
}
