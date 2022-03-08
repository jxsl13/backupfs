package backupfs

import (
	"os"
	"path/filepath"
	"time"

	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
)

var (
	// assert interfaces implemented
	_ afero.Fs        = (*HiddenFs)(nil)
	_ afero.Symlinker = (*HiddenFs)(nil)
	_ LinkOwner       = (*HiddenFs)(nil)
)

// NewHiddenFs hides away anthing beneath the specified paths.
func NewHiddenFs(base afero.Fs, hiddenPaths ...string) *HiddenFs {
	normalizedHiddenPaths := make([]string, 0, len(hiddenPaths))

	for _, p := range hiddenPaths {
		normalizedHiddenPaths = append(normalizedHiddenPaths, filepath.Clean(filepath.ToSlash(p)))
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
	base        afero.Fs
	hiddenPaths []string
}

func (fs *HiddenFs) isHidden(name string) (bool, error) {
	return internal.IsHidden(name, fs.hiddenPaths)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *HiddenFs) Create(name string) (File, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	if hidden {
		return nil, &os.PathError{Op: "create", Path: name, Err: os.ErrPermission}
	}
	return s.base.Create(name)
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *HiddenFs) Mkdir(name string, perm os.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "mkdir", Path: name, Err: os.ErrPermission}
	}
	return s.base.Mkdir(name, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *HiddenFs) MkdirAll(name string, perm os.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "mkdir_all", Path: name, Err: os.ErrPermission}
	}

	return s.base.MkdirAll(name, perm)
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (s *HiddenFs) Open(name string) (File, error) {
	return s.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (s *HiddenFs) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	if hidden {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	f, err := s.base.Open(name)
	if err != nil || f == nil {
		return nil, err
	}

	return newHiddenFsFile(f, s.hiddenPaths), nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *HiddenFs) Remove(name string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "remove", Path: name, Err: os.ErrNotExist}
	}

	return s.base.Remove(name)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *HiddenFs) RemoveAll(name string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "remove_all", Path: name, Err: os.ErrNotExist}
	}

	fi, _, err := s.LstatIfPossible(name)
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

	// directory -> walk the dir tree
	err = afero.Walk(s.base, name, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hidden, err := s.isHidden(path)
		if err != nil {
			return err
		}
		if hidden {
			// we do not touch hidden
			return nil
		}

		return s.Remove(path)
	})

	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}
	return nil
}

// Rename renames a file.
func (s *HiddenFs) Rename(oldname, newname string) error {
	hidden, err := s.isHidden(oldname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "rename", Path: oldname, Err: os.ErrNotExist}
	}

	hidden, err = s.isHidden(newname)
	if err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "rename", Path: newname, Err: os.ErrPermission}
	}

	return s.base.Rename(oldname, newname)
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *HiddenFs) Stat(name string) (os.FileInfo, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	if hidden {
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}
	return s.base.Stat(name)
}

// The name of this FileSystem
func (s *HiddenFs) Name() string {
	return "HiddenFs"
}

// Chmod changes the mode of the named file to mode.
func (s *HiddenFs) Chmod(name string, mode os.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "chmod", Path: name, Err: os.ErrNotExist}
	}

	return s.base.Chmod(name, mode)
}

// Chown changes the uid and gid of the named file.
func (s *HiddenFs) Chown(name string, uid, gid int) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "chown", Path: name, Err: os.ErrNotExist}
	}
	return s.base.Chown(name, uid, gid)
}

//Chtimes changes the access and modification times of the named file
func (s *HiddenFs) Chtimes(name string, atime, mtime time.Time) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "chtimes", Path: name, Err: os.ErrNotExist}
	}
	return s.base.Chtimes(name, atime, mtime)
}

// LstatIfPossible will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *HiddenFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: err}
	}
	if hidden {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: os.ErrNotExist}
	}

	var (
		fi          os.FileInfo
		lstatCalled = false
	)

	baseLstater, ok := internal.LstaterIfPossible(s.base)
	if ok {
		fi, lstatCalled, err = baseLstater.LstatIfPossible(name)
	} else {
		fi, err = s.base.Stat(name)
	}
	return fi, lstatCalled, err
}

//SymlinkIfPossible changes the access and modification times of the named file
func (s *HiddenFs) SymlinkIfPossible(oldname, newname string) error {
	// not allowed to symlink into hidden directory
	hidden, err := s.isHidden(oldname)
	if err != nil {
		return &os.PathError{Op: "symlink", Path: oldname, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "symlink", Path: oldname, Err: os.ErrPermission}
	}

	hidden, err = s.isHidden(newname)
	if err != nil {
		return &os.PathError{Op: "symlink", Path: newname, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "symlink", Path: newname, Err: os.ErrPermission}
	}

	if linker, ok := s.base.(afero.Linker); ok {
		err = linker.SymlinkIfPossible(oldname, newname)
		if err != nil {
			return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
		}
		return nil
	}
	return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: afero.ErrNoSymlink}
}

func (s *HiddenFs) ReadlinkIfPossible(name string) (string, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: err}
	}
	if hidden {
		return "", &os.PathError{Op: "readlink", Path: name, Err: os.ErrNotExist}
	}

	if reader, ok := s.base.(afero.LinkReader); ok {
		path, err := reader.ReadlinkIfPossible(name)
		if err != nil {
			return "", &os.PathError{Op: "readlink", Path: name, Err: err}
		}
		return path, nil
	}

	return "", &os.PathError{Op: "readlink", Path: name, Err: afero.ErrNoReadlink}
}

func (s *HiddenFs) LchownIfPossible(name string, uid, gid int) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: err}
	}
	if hidden {
		return &os.PathError{Op: "lchown", Path: name, Err: os.ErrNotExist}
	}

	linkOwner, ok := s.base.(LinkOwner)
	if !ok {
		return &os.PathError{Op: "lchown", Path: name, Err: internal.ErrNoLchown}
	}
	return linkOwner.LchownIfPossible(name, uid, gid)
}
