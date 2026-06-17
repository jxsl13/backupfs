package backupfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// foldPath lowercases a path on filesystems that are case-insensitive by
// default (Windows, macOS) so that hidden-path containment checks cannot be
// bypassed with a differently-cased path. On case-sensitive systems (Linux)
// the path is returned unchanged. Folding can only widen what is treated as
// hidden, which is the safe failure direction for backup-location protection.
// (§B B4)
func foldPath(p string) string {
	switch runtime.GOOS {
	case "windows", "darwin":
		return strings.ToLower(p)
	default:
		return p
	}
}

var (
	// assert interfaces implemented
	_ FS = (*HiddenFS)(nil)

	ErrHiddenNotExist        = fmt.Errorf("hidden: %w", os.ErrNotExist)
	ErrHiddenPermission      = fmt.Errorf("hidden: %w", fs.ErrPermission)
	wrapErrHiddenCheckFailed = func(err error) error {
		return fmt.Errorf("hidden check failed: %w", err)
	}
	wrapErrParentOfHiddenCheckFailed = func(err error) error {
		return fmt.Errorf("parent of hidden check failed: %w", err)
	}
)

// NewHiddenFS hides away anthing beneath the specified paths.
// Hidden paths MUST NOT contain a volume prefix like C:, D:, etc.
// Wrap the base filesystem in a VolumeFS if you want to target a specific volume.
func NewHiddenFS(base FS, hiddenPaths ...string) (_ *HiddenFS, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to create HiddenFS: %w", err)
		}
	}()

	// we want absolute paths in here
	normalizedHiddenPaths := make([]string, 0, len(hiddenPaths))
	for _, p := range hiddenPaths {
		normalizedPath, err := filepath.Abs(filepath.FromSlash(p))
		if err != nil {
			return nil, fmt.Errorf("failed to normalize hidden path %q: %w", p, err)
		}
		normalizedHiddenPaths = append(normalizedHiddenPaths, normalizedPath)
	}

	sort.Sort(ByMostFilePathSeparators(normalizedHiddenPaths))
	return &HiddenFS{
		base:        base,
		hiddenPaths: normalizedHiddenPaths,
	}, nil
}

// HiddenFS hides everything inside of a list of directory prefixes from the user.
// Does NOT hide the directory itself.
// This abstraction is needed in order to prevent infinite backup loops in case that
// BackupFS and HiddenFS are used together where the backup location of BackupFS is a
// prefixed path on the same underlying base filesystem (e.g. os filesystem).
// In case you want to prevent accidentally falling into an infinite recursion
// when walking and modifying files in the directory tree of a BackupFS struct
// which also contains the backup location that is modified while walking over it
// via the BackupFS layer.
//
// Writing to the hidden paths results in a fs.ErrPermission error
// Reading/Stat/Lstat from the directories or files results in os.ErrNotExist errors
type HiddenFS struct {
	base        FS
	hiddenPaths []string
}

func (fs *HiddenFS) isHidden(name string) (bool, error) {
	return isHidden(name, fs.hiddenPaths)
}

func (fs *HiddenFS) isParentOfHidden(name string) (bool, error) {
	return isParentOfHiddenDir(name, fs.hiddenPaths)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (s *HiddenFS) Create(name string) (File, error) {
	// OpenFile already does the hidden checks
	f, err := s.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	return f, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (s *HiddenFS) Mkdir(name string, perm fs.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "mkdir", Path: name, Err: ErrHiddenPermission}
	}
	err = s.base.Mkdir(name, perm)
	if err != nil {
		return err
	}
	return nil
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (s *HiddenFS) MkdirAll(name string, perm fs.FileMode) error {
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
func (s *HiddenFS) Open(name string) (File, error) {
	return s.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (s *HiddenFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
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
	if err != nil {
		return nil, err
	}

	return newHiddenFile(f, name, s.hiddenPaths), nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (s *HiddenFS) Remove(name string) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "remove", Path: name, Err: ErrHiddenNotExist}
	}

	err = s.base.Remove(name)
	if err != nil {
		return err
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (s *HiddenFS) RemoveAll(name string) error {

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
	err = Walk(s.base, name, func(path string, info fs.FileInfo, err error) error {
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
		err = s.Remove(path)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}

	// sort dirs from most nested to least nested
	// th this point all of th enon-hidden directories MUST not contain any files
	sort.Sort(ByMostFilePathSeparators(dirList))
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
func (s *HiddenFS) Rename(oldname, newname string) error {
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
		return err
	}
	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (s *HiddenFS) Stat(name string) (fs.FileInfo, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return nil, &os.PathError{Op: "stat", Path: name, Err: ErrHiddenNotExist}
	}
	fi, err := s.base.Stat(name)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

// The name of this FileSystem
func (s *HiddenFS) Name() string {
	return "HiddenFS"
}

// Chmod changes the mode of the named file to mode.
func (s *HiddenFS) Chmod(name string, mode fs.FileMode) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "chmod", Path: name, Err: ErrHiddenNotExist}
	}

	err = s.base.Chmod(name, mode)
	if err != nil {
		return err
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (s *HiddenFS) Chown(name string, uid, gid int) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "chown", Path: name, Err: ErrHiddenNotExist}
	}
	err = s.base.Chown(name, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (s *HiddenFS) Chtimes(name string, atime, mtime time.Time) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "chtimes", Path: name, Err: ErrHiddenNotExist}
	}
	err = s.base.Chtimes(name, atime, mtime)
	if err != nil {
		return err
	}
	return nil
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (s *HiddenFS) Lstat(name string) (fs.FileInfo, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: ErrHiddenNotExist}
	}
	fi, err := s.base.Lstat(name)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

// Symlink changes the access and modification times of the named file
func (s *HiddenFS) Symlink(oldname, newname string) error {
	oldname = filepath.FromSlash(oldname)
	newname = filepath.FromSlash(newname)

	var (
		hidden bool
		err    error
	)

	// not allowed to symlink into hidden directory

	if isAbs(oldname) {
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
		return err
	}
	return nil
}

func (s *HiddenFS) Readlink(name string) (string, error) {
	hidden, err := s.isHidden(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	// not allowed to read link in hidden directory
	if hidden {
		return "", &os.PathError{Op: "readlink", Path: name, Err: ErrHiddenNotExist}
	}
	link, err := s.base.Readlink(name)
	if err != nil {
		return "", err
	}
	return link, nil
}

func (s *HiddenFS) Lchown(name string, uid, gid int) error {
	hidden, err := s.isHidden(name)
	if err != nil {
		return &os.PathError{Op: "lchown", Path: name, Err: wrapErrHiddenCheckFailed(err)}
	}
	if hidden {
		return &os.PathError{Op: "lchown", Path: name, Err: ErrHiddenNotExist}
	}

	err = s.base.Lchown(name, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

func isParentOfHiddenDir(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(filepath.FromSlash(name))

	for _, hiddenDir := range hiddenPaths {
		isParentOfHiddenDir, err := dirContains(name, hiddenDir)
		if err != nil {
			return false, err
		}
		if isParentOfHiddenDir {
			return true, nil
		}

	}
	return false, nil
}

const relParent = ".." + string(os.PathSeparator)

func dirContains(parent, subdir string) (bool, error) {
	// fold case on case-insensitive filesystems so containment cannot be
	// bypassed with a differently-cased path. (§B B4)
	parent = foldPath(parent)
	subdir = foldPath(subdir)

	// Try filepath.Rel with the normalized paths first
	relPath, err := filepath.Rel(parent, subdir)
	if err != nil {
		// If that fails, try making both paths absolute (for Windows compatibility)
		var absParent, absSubdir string

		if filepath.IsAbs(parent) {
			absParent = parent
		} else {
			absParent, err = filepath.Abs(parent)
			if err != nil {
				return false, err
			}
		}

		if filepath.IsAbs(subdir) {
			absSubdir = subdir
		} else {
			absSubdir, err = filepath.Abs(subdir)
			if err != nil {
				return false, err
			}
		}

		relPath, err = filepath.Rel(absParent, absSubdir)
		if err != nil {
			return false, err
		}
	}
	relPath = filepath.FromSlash(relPath)

	isSameDir := relPath == "."
	outsideOfparentDir := strings.HasPrefix(relPath, relParent) || relPath == ".."

	return !isSameDir && !outsideOfparentDir, nil
}

func isInHiddenPath(name, hiddenDir string) (relPath string, inHiddenPath bool, err error) {
	// fold case on case-insensitive filesystems so the hidden path cannot be
	// bypassed with a differently-cased path. (§B B4)
	name = foldPath(name)
	hiddenDir = foldPath(hiddenDir)

	// Try filepath.Rel with the normalized paths first
	relPath, err = filepath.Rel(hiddenDir, name)
	if err != nil {
		// If that fails, try making both paths absolute (for Windows compatibility)
		var absName, absHiddenDir string

		if filepath.IsAbs(name) {
			absName = name
		} else {
			absName, err = filepath.Abs(name)
			if err != nil {
				return "", false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
			}
		}

		if filepath.IsAbs(hiddenDir) {
			absHiddenDir = hiddenDir
		} else {
			absHiddenDir, err = filepath.Abs(hiddenDir)
			if err != nil {
				return "", false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
			}
		}

		relPath, err = filepath.Rel(absHiddenDir, absName)
		if err != nil {
			return "", false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
		}
	}

	relPath = filepath.FromSlash(relPath)

	// no ../ prefix
	// -> does not lie outside of hidden dir
	outsideOfHiddenDir := strings.HasPrefix(relPath, relParent)
	isParentDir := relPath == ".."
	isHiddenDir := relPath == "."

	if !isHiddenDir && (outsideOfHiddenDir || isParentDir) {
		return relPath, false, nil
	}

	return relPath, true, nil
}

// hiddenPaths should be normalized (filepath.Clean result values)
func isHidden(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	absName, err := filepath.Abs(filepath.FromSlash(name))
	if err != nil {
		return false, fmt.Errorf("failed to make path absolute %s: %w", name, err)
	}

	for _, hiddenDir := range hiddenPaths {
		_, hidden, err := isInHiddenPath(absName, hiddenDir)
		if err != nil {
			return false, err
		}
		if hidden {
			return true, nil
		}
	}
	return false, nil
}

func allFiles(fsys FS, dir string) ([]string, error) {
	files := make([]string, 0)

	err := Walk(fsys, dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
