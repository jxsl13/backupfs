package backupfs

import (
	"fmt"
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

type prefixFSOptions struct {
	// EnableSymlinkEscape makes symlinks point outside of the filesystem abstraction folder
	EnableSymlinkEscape bool
}

type PrefixFSOption func(*prefixFSOptions) error

// PrefixFSWithAllowSymlinkEscape sets whether symlinks are allowed to point outside of the PrefixFS.
// When this option is enabled, all absolute symlink paths will point directly into the root filesystem.
func PrefixFSWithEnableSymlinkEscape(enable bool) PrefixFSOption {
	return func(o *prefixFSOptions) error {
		o.EnableSymlinkEscape = enable
		return nil
	}
}

// NewPrefixFS creates a new file system abstraction that forces any path to be prepended with
// the provided prefix.
// the existence of the prefixPath existing is hidden away (errors might show full paths).
// The prefixPath is seen as the root directory.
// The prefix path MUST NOT contain a Windows (OS) volume prefix like C:, D:, etc.
// Wrap the base filesystem in a VolumeFS if you want to target a specific volume.
func NewPrefixFS(fsys FS, prefixPath string, opts ...PrefixFSOption) (_ *PrefixFS, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to create PrefixFS: %w", err)
		}
	}()
	prefixPath = filepath.FromSlash(prefixPath)

	options := prefixFSOptions{}

	// unwrap other PrefixFS - required in order to properly support Windows
	// nested drive letters
	if pfs, ok := fsys.(*PrefixFS); ok {
		fsys = pfs.base
		prefixPath = normalizeVolumePath(prefixPath)
		prefixPath = filepath.Join(pfs.prefix, prefixPath)

		// inherit options from unwrapped PrefixFS
		options = pfs.opts
	}

	for _, o := range opts {
		err = o(&options)
		if err != nil {
			return nil, err
		}
	}

	absPrefixPath, err := filepath.Abs(prefixPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create absolute path from provided path %s: %w", prefixPath, err)
	}

	return &PrefixFS{
		prefix: absPrefixPath,
		base:   fsys,
		opts:   options,
	}, nil
}

// PrefixFS, contrary to BasePathFS, does abstract away the existence of a base path.
// The prefixed path is seen as the root directory.
type PrefixFS struct {
	prefix string
	base   FS
	opts   prefixFSOptions
}

// c:\\test -> \\c\\test
func normalizeVolumePath(p string) string {
	volume := filepath.VolumeName(p)
	if volume != "" {
		// interesting for windows, as this backup mechanism does not exactly work
		// with prefixed directories otherwise. A colon is not allowed inisde of the file path.
		// prefix path with volume letter but without the :
		volumeName := strings.TrimRight(volume, ":")
		nameWithoutVolume := p[len(volume):]
		return filepath.Join(separator+strings.ToUpper(volumeName), nameWithoutVolume)
	}
	return p
}

func (s *PrefixFS) prefixPath(name string) (absolute, prefixed string, err error) {

	absolute, err = filepath.Abs(filepath.FromSlash(name))
	if err != nil {
		return "", "", fmt.Errorf("failed to make path absolute %s: %w", name, err)
	}
	prefixed = normalizeVolumePath(absolute)
	prefixed = filepath.Join(s.prefix, prefixed)
	if !strings.HasPrefix(prefixed, s.prefix) {
		return "", "", syscall.EPERM
	}
	return absolute, prefixed, nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *PrefixFS) Create(name string) (File, error) {
	abs, prefixed, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: err}
	}
	f, err := s.base.Create(prefixed)
	if err != nil {
		return nil, err
	}

	return newPrefixFile(f, name, abs), nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *PrefixFS) Mkdir(name string, perm fs.FileMode) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}
	err = s.base.Mkdir(prefixed, perm)
	if err != nil {
		return err
	}
	return nil
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *PrefixFS) MkdirAll(name string, perm fs.FileMode) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir_all", Path: name, Err: err}
	}

	err = s.base.MkdirAll(prefixed, perm)
	if err != nil {
		return err
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *PrefixFS) Open(name string) (File, error) {
	abs, prefixed, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	f, err := s.base.Open(prefixed)
	if err != nil {
		return nil, err
	}

	return newPrefixFile(f, name, abs), nil
}

// OpenFile opens a file using the given flags and the given mode.
func (s *PrefixFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	abs, prefixed, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open_file", Path: name, Err: err}
	}

	f, err := s.base.OpenFile(prefixed, flag, perm)
	if err != nil {
		return nil, err
	}

	return newPrefixFile(f, name, abs), nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *PrefixFS) Remove(name string) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	// Prevent removing the root directory of the PrefixFS
	// This would make the filesystem inconsistent
	if prefixed == s.prefix {
		return &fs.PathError{Op: "remove", Path: name, Err: syscall.EPERM}
	}

	err = s.base.Remove(prefixed)
	if err != nil {
		return err
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *PrefixFS) RemoveAll(name string) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// Prevent removing the root directory of the PrefixFS
	// This would make the filesystem inconsistent
	if prefixed == s.prefix {
		return &fs.PathError{Op: "remove_all", Path: name, Err: syscall.EPERM}
	}

	err = s.base.RemoveAll(prefixed)
	if err != nil {
		return err
	}
	return nil
}

// Rename renames a file.
func (s *PrefixFS) Rename(oldname, newname string) error {
	_, oldpath, err := s.prefixPath(oldname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: oldname, Err: err}
	}

	// Prevent renaming the root directory of the PrefixFS
	// This would make the filesystem inconsistent
	if oldpath == s.prefix {
		return &fs.PathError{Op: "rename", Path: oldname, Err: syscall.EPERM}
	}

	_, newpath, err := s.prefixPath(newname)
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
	abs, prefixed, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	fi, err := s.base.Stat(prefixed)
	if err != nil {
		return nil, err
	}

	return newPrefixFileInfo(fi, abs), nil
}

// The name of this FileSystem
func (s *PrefixFS) Name() string {
	return "PrefixFS"
}

// Chmod changes the mode of the named file to mode.
func (s *PrefixFS) Chmod(name string, mode fs.FileMode) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return err
	}

	err = s.base.Chmod(prefixed, mode)
	if err != nil {
		return err
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (s *PrefixFS) Chown(name string, uid, gid int) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chown", Path: name, Err: err}
	}

	err = s.base.Chown(prefixed, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (s *PrefixFS) Chtimes(name string, atime, mtime time.Time) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chtimes", Path: name, Err: err}
	}
	err = s.base.Chtimes(prefixed, atime, mtime)
	if err != nil {
		return err
	}
	return nil
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *PrefixFS) Lstat(name string) (fs.FileInfo, error) {
	absolute, prefixed, err := s.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}

	fi, err := s.base.Lstat(prefixed)
	if err != nil {
		return nil, err
	}
	return newPrefixFileInfo(fi, absolute), nil
}

// Symlink changes the access and modification times of the named file
// On Windows root relative symlinks ( \\User\\abs\\...) will be converted to relative (..\\abc\\... ) symlinks.

func (s *PrefixFS) Symlink(oldname, newname string) (err error) {
	defer func() {
		if err != nil {
			err = &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
		}
	}()
	// links may be relative paths
	newAbs, newPathPrefixed, err := s.prefixPath(newname)
	if err != nil {
		return err
	}

	if s.opts.EnableSymlinkEscape {
		// make symlinks point outside of the prefix fs
		// just create the symlink as is but inside the prefixed fs
		err = s.base.Symlink(oldname, newPathPrefixed)
		if err != nil {
			return err
		}
		return nil
	} else if isAbs(oldname) {
		_, oldPathPrefixed, err := s.prefixPath(oldname)
		if err != nil {
			return err
		}
		err = s.base.Symlink(oldPathPrefixed, newPathPrefixed)
		if err != nil {
			return err
		}
		return nil
	}

	// symlink is relative
	newAbsDirPath := filepath.Dir(newAbs)
	oldAbs := filepath.Join(newAbsDirPath, oldname)

	_, _, err = s.prefixPath(oldAbs)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(newAbsDirPath, oldAbs)
	if err != nil {
		return err
	}

	err = s.base.Symlink(rel, newPathPrefixed)
	if err != nil {
		return err
	}
	return nil
}

// Readlink only returns absolute or relative paths, never root relative paths.
func (s *PrefixFS) Readlink(name string) (_ string, err error) {
	defer func() {
		if err != nil {
			err = &fs.PathError{Op: "readlink", Path: name, Err: err}
		}
	}()
	_, prefixedNewPath, err := s.prefixPath(name)
	if err != nil {
		return "", err
	}

	oldname, err := s.base.Readlink(prefixedNewPath)
	if err != nil {
		return "", err
	}
	cleanedOldname := filepath.Clean(filepath.FromSlash(oldname))

	if isAbs(cleanedOldname) {
		return reconstructVolume(cleanedOldname, s.prefix), nil
	}

	return cleanedOldname, nil
}

func (s *PrefixFS) Lchown(name string, uid, gid int) error {
	_, prefixed, err := s.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "lchown", Path: name, Err: err}
	}

	err = s.base.Lchown(prefixed, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

func reconstructVolume(absOldPath, prefix string) string {
	// reconstruct drive letter if volumes are available
	if filepath.VolumeName(absOldPath) == "" {
		return strings.TrimPrefix(absOldPath, prefix)
	}

	// windows handling
	// we get a path like this /c/Users/name/...
	volumePath := strings.TrimLeft(strings.TrimPrefix(absOldPath, prefix), separator)
	// now we need to reconstruct the drive letter
	parts := strings.SplitN(volumePath, separator, 2)
	switch len(parts) {
	case 0:
		return ""
	case 1:
		// just a drive letter (root path)
		return filepath.Clean(parts[0] + ":" + separator)
	default:
		// reconstruct drive letter with trailing path
		return filepath.Clean(parts[0] + ":" + separator + parts[1])
	}
}

// PrefixPath joins two paths together, making sure the resulting path is absolute
// and normalized (volume names are converted to root relative paths).
// On Windows the volume name is preserved, but the colon is removed.
func PrefixPath(prefix, name string) (string, error) {
	name = filepath.FromSlash(name)
	absolute, err := filepath.Abs(filepath.FromSlash(name))
	if err != nil {
		return "", fmt.Errorf("failed to make path absolute %s: %w", name, err)
	}
	absolute = normalizeVolumePath(absolute)
	absolute = filepath.Join(prefix, absolute)
	return absolute, nil
}
