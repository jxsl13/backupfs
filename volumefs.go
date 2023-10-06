package backupfs

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxsl13/backupfs/fsi"
)

// assert interfaces implemented
var (
	_ fsi.Fs = (*VolumeFs)(nil)
)

type volumeFile = PrefixFile
type volumeFileInfo = prefixFileInfo

// VolumeFs is specifically designed to prefix absolute paths with a defined volume like C:, D:, E: etc.
// We want to be able to decide which volume to target on Windows operating systems.
type VolumeFs struct {
	volume string
	base   fsi.Fs
}

// the passed file path must not contain any os specific volume prefix.
// primarily no windows volumes like c:, d:, etc.
func (v *VolumeFs) prefixPath(name string) (string, error) {
	name = filepath.Clean(name)

	if v.volume == "" {
		return name, nil
	}

	volumePrefix := filepath.VolumeName(name)
	if volumePrefix != "" {
		return "", os.ErrNotExist
	}

	return filepath.Clean(filepath.Join(v.volume, name)), nil
}

func NewVolumeFs(volume string, fs fsi.Fs) *VolumeFs {
	return &VolumeFs{
		volume: filepath.VolumeName(volume), // this part behaves differently depending on the operating system
		base:   fs,
	}
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (v *VolumeFs) Create(name string) (fsi.File, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, err
	}

	f, err := v.base.Create(path)
	if f == nil {
		return nil, err
	}

	return &volumeFile{f: f, prefix: v.volume}, nil // TODO: do we need an own file type
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (v *VolumeFs) Mkdir(name string, perm os.FileMode) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.Mkdir(path, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (v *VolumeFs) MkdirAll(name string, perm os.FileMode) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.MkdirAll(path, perm)
}

// Open opens a file, returning it or an error, if any happens.
// This returns a ready only file
func (v *VolumeFs) Open(name string) (fsi.File, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, err
	}

	f, err := v.base.Open(path)
	if f == nil {
		return nil, err
	}

	return &volumeFile{f: f, prefix: v.volume}, nil
}

// OpenFile opens a file using the given flags and the given mode.
func (v *VolumeFs) OpenFile(name string, flag int, perm os.FileMode) (fsi.File, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, err
	}

	f, err := v.base.OpenFile(path, flag, perm)
	if f == nil {
		return nil, err
	}

	return &volumeFile{f: f, prefix: v.volume}, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (v *VolumeFs) Remove(name string) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.Remove(path)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (v *VolumeFs) RemoveAll(name string) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.RemoveAll(path)
}

// Rename renames a file.
func (v *VolumeFs) Rename(oldname, newname string) error {
	oldpath, err := v.prefixPath(oldname)
	if err != nil {
		return err
	}
	newpath, err := v.prefixPath(newname)
	if err != nil {
		return err
	}

	return v.base.Rename(oldpath, newpath)
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (v *VolumeFs) Stat(name string) (fs.FileInfo, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, err
	}

	fi, err := v.base.Stat(path)
	if err != nil {
		return nil, err
	}

	return &volumeFileInfo{fi, v.volume}, nil
}

// The name of this FileSystem
func (v *VolumeFs) Name() string {
	return "VolumeFs"
}

// Chmod changes the mode of the named file to mode.
func (v *VolumeFs) Chmod(name string, mode os.FileMode) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.Chmod(path, mode)
}

// Chown changes the uid and gid of the named file.
func (v *VolumeFs) Chown(name string, username, group string) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.Chown(path, username, group)
}

// Chtimes changes the access and modification times of the named file
func (v *VolumeFs) Chtimes(name string, atime, mtime time.Time) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}
	return v.base.Chtimes(path, atime, mtime)
}

// Lstat will call Lstat if the filesystem itself is, or it delegates to, the os filesystem.
// Else it will call Stat.
// In addtion to the FileInfo, it will return a boolean telling whether Lstat was called or not.
func (v *VolumeFs) Lstat(name string) (fs.FileInfo, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return nil, err
	}

	return v.base.Lstat(path)
}

// Symlink changes the access and modification times of the named file
func (v *VolumeFs) Symlink(oldname, newname string) error {
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
		oldPath = oldname
	}

	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err}
	}

	newPath, err := v.prefixPath(newname)
	if err != nil {
		return err
	}

	return v.base.Symlink(oldPath, newPath)
}

func (v *VolumeFs) Readlink(name string) (string, error) {
	path, err := v.prefixPath(name)
	if err != nil {
		return "", err
	}

	linkedPath, err := v.base.Readlink(path)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(linkedPath, v.volume), nil
}

func (v *VolumeFs) Lchown(name string, username, group string) error {
	path, err := v.prefixPath(name)
	if err != nil {
		return err
	}

	return v.base.Lchown(path, username, group)
}

// TrimVolume trims the volume prefix of a given filepath. C:\A\B\C -> \A\B\C
// highly OS-dependent. On unix systems there is no such thing as a volume path prefix.
func TrimVolume(filePath string) string {
	volume := filepath.VolumeName(filePath)
	return filePath[len(volume):]
}
