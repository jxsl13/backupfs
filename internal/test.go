package internal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
	"github.com/stretchr/testify/require"
)

func CreateMemDir(path string, perm os.FileMode) os.FileInfo {
	path = filepath.Clean(path)

	fd := mem.CreateDir(path)
	mem.SetMode(fd, os.ModeDir|perm)
	return mem.GetFileInfo(fd)
}

func CreateMemFile(path, content string, perm os.FileMode) afero.File {
	path = filepath.Clean(path)

	fd := mem.CreateFile(path)
	mem.SetMode(fd, perm)
	return mem.NewFileHandle(fd)
}

func FileMustContainText(t *testing.T, fs afero.Fs, path, content string) {
	path = filepath.Clean(path)

	require := require.New(t)
	f, err := fs.Open(path)
	require.NoError(err)
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	require.NoError(err)

	require.Equal(string(b), content)
}

func SymlinkMustExist(t *testing.T, fs afero.Fs, symlinkPath string) {
	symlinkPath = filepath.Clean(symlinkPath)

	require := require.New(t)
	sf, ok := fs.(SymlinkFs)
	require.Truef(ok, "filesystem does not implement the SymlinkFs interface: %s", fs.Name())

	fi, lstatCalled, err := sf.LstatIfPossible(symlinkPath)
	require.Falsef(os.IsNotExist(err), "target symlink does not exist but is expected to exist: %s", symlinkPath)

	require.NoError(err)

	require.Truef(lstatCalled, "lstat has no been called for: %s", symlinkPath)

	hasSymlinkFlag := fi.Mode().Type()&os.ModeSymlink != 0
	require.Truef(hasSymlinkFlag, "target symlink does not have the symlink flag: %s", symlinkPath)

	actualPointsTo, err := sf.ReadlinkIfPossible(symlinkPath)
	require.NoError(err)

	require.True(actualPointsTo != "", "symlink target path is empty")
}

func SymlinkMustExistWithTragetPath(t *testing.T, fs afero.Fs, symlinkPath, expectedPointsTo string) {
	symlinkPath = filepath.Clean(symlinkPath)
	expectedPointsTo = filepath.Clean(expectedPointsTo)

	require := require.New(t)
	sf, ok := fs.(SymlinkFs)
	require.True(ok, "filesystem does not implement the SymlinkFs interface: ", fs.Name())

	fi, lstatCalled, err := sf.LstatIfPossible(symlinkPath)
	require.False(os.IsNotExist(err), "target symlink does not exist but is expected to exist: ", symlinkPath)

	require.NoError(err)

	require.True(lstatCalled, "lstat has no been called for: ", symlinkPath)

	hasSymlinkFlag := fi.Mode().Type()&os.ModeSymlink != 0
	require.True(hasSymlinkFlag, "target symlink does not have the symlink flag: ", symlinkPath)

	actualPointsTo, err := sf.ReadlinkIfPossible(symlinkPath)
	require.NoError(err)

	require.Equal(expectedPointsTo, actualPointsTo, "symlink does not point to the expected path")

}

func MustNotExist(t *testing.T, fs afero.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := Exists(fs, path)
	require.NoError(err)
	require.False(found, "found file path but should not exist: "+path)
}

func MustExist(t *testing.T, fs afero.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := Exists(fs, path)
	require.NoError(err)
	require.True(found, "file path not found but should exist: "+path)
}

func MustLExist(t *testing.T, fs afero.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := LExists(fs, path)
	require.NoError(err)
	require.Truef(found, "symlink path not found but should exist: %s", path)
}

func RemoveFile(t *testing.T, fs afero.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)

	err := fs.Remove(path)
	require.NoError(err)
}

func RemoveAll(t *testing.T, fs afero.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)

	err := fs.RemoveAll(path)
	require.NoError(err)

	MustNotExist(t, fs, path)
}

type SymlinkFs interface {
	afero.Fs
	afero.Symlinker
}

func CreateSymlink(t *testing.T, fs afero.Fs, oldpath, newpath string) {
	require := require.New(t)

	sf, ok := fs.(SymlinkFs)
	require.Truef(ok, "filesystem does not implement the SymlinkFs interface: %s", fs.Name())

	oldpath = filepath.Clean(oldpath)
	newpath = filepath.Clean(newpath)

	dirPath := filepath.Dir(oldpath)
	found, err := Exists(sf, dirPath)
	require.NoError(err)

	if !found {
		err = sf.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	dirPath = filepath.Dir(newpath)
	found, err = Exists(sf, dirPath)
	require.NoError(err)

	if !found {
		err = sf.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	// check newpath after creating the symlink
	err = sf.SymlinkIfPossible(oldpath, newpath)
	require.NoError(err)

	fi, lstatCalled, err := sf.LstatIfPossible(newpath)
	require.NoError(err)

	require.Truef(lstatCalled, "lstat has not been called but is expected to have been called (old -> new): %s -> %s", oldpath, newpath)

	hasSymlinkFlag := fi.Mode().Type()&os.ModeSymlink != 0
	require.True(hasSymlinkFlag, "the target(newpath) symlink does not have the symlink flag set: ", newpath)

	// check oldpath after creating the symlink
	fi, lstatCalled, err = sf.LstatIfPossible(oldpath)
	require.NoError(err)

	require.True(lstatCalled, "lstat has not been called but is expected to have been called (old -> new): %s -> %s", oldpath, newpath)

	hasSymlinkFlag = fi.Mode().Type()&os.ModeSymlink != 0
	require.Falsef(hasSymlinkFlag, "the source (oldpath) symlink does have the symlink flag set but is expected not to have it set: %s", oldpath)
}

func CreateFile(t *testing.T, fs afero.Fs, path, content string) {
	path = filepath.Clean(path)

	require := require.New(t)

	dirPath := filepath.Dir(path)
	found, err := Exists(fs, dirPath)
	require.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	f, err := fs.Create(path)
	require.NoError(err)
	defer func(file afero.File) {
		err := f.Close()
		require.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	require.NoError(err)
	require.Equal(ret, len(content))
}

func OpenFile(t *testing.T, fs afero.Fs, path, content string, perm os.FileMode) {
	path = filepath.Clean(path)

	require := require.New(t)

	dirPath := filepath.Dir(path)
	found, err := Exists(fs, dirPath)
	require.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	f, err := fs.OpenFile(path, os.O_RDWR|os.O_TRUNC|os.O_CREATE, perm)
	require.NoError(err)
	defer func(file afero.File) {
		err := f.Close()
		require.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	require.NoError(err)
	require.Equal(ret, len(content))
}

func MkdirAll(t *testing.T, fs afero.Fs, path string, perm os.FileMode) {
	path = filepath.Clean(path)

	require := require.New(t)
	err := fs.MkdirAll(path, perm)
	require.NoError(err)

	err = IterateDirTree(path, func(s string) error {
		exists, err := Exists(fs, s)
		if err != nil {
			return err
		}
		require.True(exists, "path not found but is expected to exist: ", s)
		return nil
	})
	require.NoError(err)
}
