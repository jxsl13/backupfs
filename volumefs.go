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
	_ FS = (*VolumeFS)(nil)
)

type volumeFile = prefixFile
type volumeFileInfo = prefixFileInfo

// VolumeFS is specifically designed to prefix absolute paths with a defined volume like C:, D:, E: etc.
// We want to be able to decide which volume to target on Windows operating systems.
type VolumeFS struct {
	volume string
	base   FS
}

// the passed file path must not contain any os specific volume prefix.
// primarily no windows volumes like c:, d:, etc.
func (v *VolumeFS) prefixPath(name string) (string, error) {
	name = filepath.Clean(name)

	if v.volume == "" {
		return name, nil
	}

	volumePrefix := filepath.VolumeName(name)
	if volumePrefix != "" {
		return "", syscall.EPERM
	}

	return filepath.Clean(filepath.Join(v.volume, name)), nil
}

func NewVolumeFS(volume string, fs FS) *VolumeFS {
	return &VolumeFS{
		volume: filepath.VolumeName(volume), // this part behaves differently depending on the operating system
		base:   fs,
	}
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (v *VolumeFS) Create(name string) (File, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: err}
	}

	f, err := v.base.Create(path)
	if err != nil {
		return nil, err
	}

	return &volumeFile{f: f, prefix: v.volume}, nil // TODO: do we need an own file type
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (v *VolumeFS) Mkdir(name string, perm fs.FileMode) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}

	err = v.base.Mkdir(path, perm)
	if err != nil {
		return err
	}
	return nil
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (v *VolumeFS) MkdirAll(name string, perm fs.FileMode) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir_all", Path: name, Err: err}
	}

	err = v.base.MkdirAll(path, perm)
	if err != nil {
		return err
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (v *VolumeFS) Open(name string) (File, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	f, err := v.base.Open(path)
	if err != nil {
		return nil, err
	}

	return &volumeFile{f: f, prefix: v.volume}, nil
}

// OpenFile opens a file using the given flags and the given mode.
func (v *VolumeFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open_file", Path: name, Err: err}
	}

	f, err := v.base.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

	return &volumeFile{f: f, prefix: v.volume}, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (v *VolumeFS) Remove(name string) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	err = v.base.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (v *VolumeFS) RemoveAll(name string) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "remove_all", Path: name, Err: err}
	}

	err = v.base.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}

// Rename renames a file.
func (v *VolumeFS) Rename(oldname, newname string) error {
	oldpath, err := v.prefixPath(oldname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: err}
	}
	newpath, err := v.prefixPath(newname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: err}
	}

	err = v.base.Rename(oldpath, newpath)
	if err != nil {
		return err
	}
	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (v *VolumeFS) Stat(name string) (fs.FileInfo, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	fi, err := v.base.Stat(path)
	if err != nil {
		return nil, err
	}

	return &volumeFileInfo{fi, v.volume}, nil
}

// The name of this FileSystem
func (v *VolumeFS) Name() string {
	return "VolumeFS"
}

// Chmod changes the mode of the named file to mode.
func (v *VolumeFS) Chmod(name string, mode fs.FileMode) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chmod", Path: name, Err: err}
	}

	err = v.base.Chmod(path, mode)
	if err != nil {
		return err
	}
	return nil
}

// Chown changes the uid and gid of the named file.
func (v *VolumeFS) Chown(name string, uid, gid int) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chown", Path: name, Err: err}
	}

	err = v.base.Chown(path, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// Chtimes changes the access and modification times of the named file
func (v *VolumeFS) Chtimes(name string, atime, mtime time.Time) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "chtimes", Path: name, Err: err}
	}
	err = v.base.Chtimes(path, atime, mtime)
	if err != nil {
		return err
	}
	return nil
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (v *VolumeFS) Lstat(name string) (fs.FileInfo, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}

	fi, err := v.base.Lstat(path)
	if err != nil {
		return nil, err
	}
	return newPrefixFileInfo(fi, v.volume), nil

}

// Symlink changes the access and modification times of the named file
func (v *VolumeFS) Symlink(oldname, newname string) error {
	// links may be relative paths

	var (
		err     error
		oldPath string
	)
	if path.IsAbs(filepath.ToSlash(oldname)) || filepath.IsAbs(filepath.FromSlash(oldname)) {
		// absolute path symlink
		oldPath, err = v.prefixPath(oldname)
	} else {
		// relative path symlink
		// TODO: oldname could escape the volume prefix using relative paths
		oldPath = oldname
	}

	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	newPath, err := v.prefixPath(newname)
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	err = v.base.Symlink(oldPath, newPath)
	if err != nil {
		return err
	}
	return nil
}

func (v *VolumeFS) Readlink(name string) (string, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}

	linkedPath, err := v.base.Readlink(path)
	if err != nil {
		return "", err
	}

	cleanedPath := filepath.Clean(linkedPath)
	return strings.TrimPrefix(cleanedPath, v.volume), nil
}

func (v *VolumeFS) Lchown(name string, uid, gid int) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return &fs.PathError{Op: "lchown", Path: name, Err: err}
	}
	err = v.base.Lchown(path, uid, gid)
	if err != nil {
		return err
	}
	return nil
}

// TrimVolume trims the volume prefix of a given filepath. C:\A\B\C -> \A\B\C
// highly OS-dependent. On unix systems there is no such thing as a volume path prefix.
func TrimVolume(filePath string) string {
	volume := filepath.VolumeName(filePath)
	return filePath[len(volume):]
}
