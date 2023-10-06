package backupfs

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/jxsl13/backupfs/fsi"
	"github.com/jxsl13/backupfs/fsutils"
	"github.com/jxsl13/backupfs/internal"
)

var (
	// assert interfaces implemented
	_ fsi.Fs = (*HiddenFs)(nil)

	ErrHiddenNotExist        = fmt.Errorf("hidden: %w", os.ErrNotExist)
	ErrHiddenPermission      = fmt.Errorf("hidden: %w", os.ErrPermission)
	wrapErrHiddenCheckFailed = func(err error) error {
		return fmt.Errorf("hidden check failed: %w", err)
	}
	wrapErrParentOfHiddenCheckFailed = func(err error) error {
		return fmt.Errorf("parent of hidden check failed: %w", err)
	}
)

// NewHiddenFs hides away anthing beneath the specified paths.
func NewHiddenFs(base fsi.Fs, hiddenPaths ...string) *HiddenFs {
	normalizedHiddenPaths := make([]string, 0, len(hiddenPaths))

	for _, p := range hiddenPaths {
		normalizedHiddenPaths = append(normalizedHiddenPaths, filepath.Clean(filepath.FromSlash(p)))
	}

	return &HiddenFs{
		base:        base,
		hiddenPaths: normalizedHiddenPaths,
	}
}

// HiddenFs hides everything inside of a list of directory prefixes from the user.
// Does NOT hide the directory itself.
// This abstraction is needed in order to prevent infinite backup loops in case that
// BackupFs and HiddenFs are used together where the backup location of BackupFs is a
// prefixed path on the same underlying base filesystem (e.g. os filesystem).
// In case you want to prevent accidentally falling into an infinite recursion
// when walking and modifying files in the directory tree of a BackupFs struct
// which also contains the backup location that is modified while walking over it
// via the BackupFs layer.
//
// Writing to the hidden paths results in a os.ErrPermission error
// Reading/Stat/Lstat from the directories or files results in os.ErrNotExist errors
type HiddenFs struct {
	base        fsi.Fs
	hiddenPaths []string
}

func (fs *HiddenFs) isHidden(name string) (bool, error) {
	return internal.IsHidden(name, fs.hiddenPaths)
}

func (fs *HiddenFs) isParentOfHidden(name string) (bool, error) {
	return internal.IsParentOfHiddenDir(name, fs.hiddenPaths)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *HiddenFs) Create(name string) (fsi.File, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return nil, &os.PathError{Op: "create", Path: name, Err: ErrHiddenPermission}
	}
	f, err := s.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	return f, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *HiddenFs) Mkdir(name string, perm os.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "mkdir", Path: name, Err: ErrHiddenPermission}
	}
	return s.base.Mkdir(name, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *HiddenFs) MkdirAll(name string, perm os.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: ErrHiddenPermission}
	}

	return s.base.MkdirAll(name, perm)
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *HiddenFs) Open(name string) (fsi.File, error) {
	return s.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (s *HiddenFs) OpenFile(name string, flag int, perm os.FileMode) (fsi.File, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		if flag&os.O_CREATE != 0 {
			// requesting creation
			return nil, &os.PathError{Op: "open", Path: name, Err: ErrHiddenPermission}
		}
		// requesting access
		return nil, &os.PathError{Op: "open", Path: name, Err: ErrHiddenNotExist}
	}
	f, err := s.base.OpenFile(name, flag, perm)
	if err != nil || f == nil {
		return nil, err
	}

	return newHiddenFsFile(f, name, s.hiddenPaths), nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *HiddenFs) Remove(name string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "remove", Path: name, Err: ErrHiddenNotExist}
	}

	return s.base.Remove(name)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *HiddenFs) RemoveAll(name string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "remove_all", Path: name, Err: ErrHiddenNotExist}
	}

	fi, err := s.Lstat(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// if it's a file or a symlink, directly remove it
	if !fi.IsDir() {
		err = s.Remove(name)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: err}
		}
		return nil
	}

	// will contain only non-hidden directories
	dirList := make([]string, 0, 2)

	// directory -> walk the dir tree
	err = fsutils.Walk(s.base, name, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hidden, err := s.isHidden(path)
		if err != nil {
			return wrapErrHiddenCheckFailed(err)
		}
		// skip hidden files
		if hidden {
			// we do not touch hidden
			return nil
		}

		if info.IsDir() {
			// dirs will be handled after all of the other files
			dirList = append(dirList, path)
			return nil
		}

		// file or symlink or whatever else
		return s.Remove(path)
	})
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// sort dirs from most nested to least nested
	// th this point all of th enon-hidden directories MUST not contain any files
	sort.Sort(internal.ByMostFilePathSeparators(dirList))
	for _, dir := range dirList {
		containsHidden, err := s.isParentOfHidden(dir)
		if err != nil {
			return &os.PathError{Op: "remove_all", Path: name, Err: wrapErrParentOfHiddenCheckFailed(err)}
		}

		if !containsHidden {
			err = s.base.Remove(dir)
			if err != nil {
				return &os.PathError{Op: "remove_all", Path: name, Err: err}
			}
		}
	}

	return nil
}

// Rename renames a file.
func (s *HiddenFs) Rename(oldname, newname string) error {
	hidden, err := s.isHidden(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "rename", Path: oldname, Err: ErrHiddenNotExist}
	}

	hidden, err = s.isHidden(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "rename", Path: newname, Err: ErrHiddenPermission}
	}

	err = s.base.Rename(oldname, newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: wrapErrHiddenCheckFailed(err)}
	}
	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *HiddenFs) Stat(name string) (fs.FileInfo, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return nil, &os.PathError{Op: "stat", Path: name, Err: ErrHiddenNotExist}
	}
	fi, err := s.base.Stat(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return fi, nil
}

// The name of this FileSystem
func (s *HiddenFs) Name() string {
	return "HiddenFs"
}

// Chmod changes the mode of the named file to mode.
func (s *HiddenFs) Chmod(name string, mode os.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "chmod", Path: name, Err: ErrHiddenNotExist}
	}

	err = s.base.Chmod(name, mode)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (s *HiddenFs) Chown(name string, username, group string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "chown", Path: name, Err: ErrHiddenNotExist}
	}
	err = s.base.Chown(name, username, group)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (s *HiddenFs) Chtimes(name string, atime, mtime time.Time) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "chtimes", Path: name, Err: ErrHiddenNotExist}
	}
	err = s.base.Chtimes(name, atime, mtime)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return nil
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
func (s *HiddenFs) Lstat(name string) (fs.FileInfo, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: ErrHiddenNotExist}
	}

	fi, err := s.base.Lstat(name)
	if err != nil {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return fi, nil
}

// Symlink changes the access and modification times of the named file
func (s *HiddenFs) Symlink(oldname, newname string) error {
	oldname = filepath.FromSlash(oldname)
	newname = filepath.FromSlash(newname)

	var (
		hidden bool
		err    error
	)

	// not allowed to symlink into hidden directory

	if path.IsAbs(filepath.ToSlash(oldname)) || filepath.IsAbs(filepath.FromSlash(oldname)) {
		hidden, err = s.isHidden(oldname)
	} else {
		startingDir := filepath.Dir(newname)
		hidden, err = s.isHidden(filepath.Join(startingDir, oldname))
	}

	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {

		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: ErrHiddenPermission}
	}

	// no allowed to create symlink in hidden directory
	hidden, err = s.isHidden(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: ErrHiddenPermission}
	}

	err = s.base.Symlink(oldname, newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: wrapErrHiddenCheckFailed(err)}
	}
	return nil
}

func (s *HiddenFs) Readlink(name string) (string, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	// not allowed to read link in hidden directory
	if hidden {
		return "", &os.PathError{Op: "readlink", Path: name, Err: ErrHiddenNotExist}
	}

	path, err := s.base.Readlink(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return path, nil
}

func (s *HiddenFs) Lchown(name string, username, group string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "lchown", Path: name, Err: ErrHiddenNotExist}
	}

	err = s.base.Lchown(name, username, group)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	return nil
}
