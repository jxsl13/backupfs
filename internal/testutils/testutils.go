package testutils

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jxsl13/backupfs/fsutils"
	"github.com/jxsl13/backupfs/interfaces"
	"github.com/jxsl13/backupfs/internal"
	"github.com/jxsl13/backupfs/mem"
	"github.com/stretchr/testify/require"
)

func CreateMemDir(path string, perm os.FileMode) fs.FileInfo {
	path = filepath.Clean(path)

	fd := mem.CreateDir(path)
	mem.SetMode(fd, os.ModeDir|perm)
	return mem.GetFileInfo(fd)
}

func CreateMemFile(path, content string, perm os.FileMode) interfaces.File {
	path = filepath.Clean(path)

	fd := mem.CreateFile(path)
	mem.SetMode(fd, perm)
	return mem.NewFileHandle(fd)
}

func FileMustContainText(t *testing.T, fs interfaces.Fs, path, content string) {
	path = filepath.Clean(path)

	require := require.New(t)
	f, err := fs.Open(path)
	require.NoError(err)
	defer f.Close()
	b, err := io.ReadAll(f)
	require.NoError(err)

	require.Equal(string(b), content)
}

func SymlinkMustExist(t *testing.T, ifs interfaces.Fs, symlinkPath string) {
	symlinkPath = filepath.Clean(symlinkPath)

	require := require.New(t)

	fi, err := ifs.Lstat(symlinkPath)
	require.Falsef(errors.Is(err, fs.ErrNotExist), "target symlink does not exist but is expected to exist: %s", symlinkPath)

	require.NoError(err)

	hasSymlinkFlag := fi.Mode()&os.ModeType&os.ModeSymlink != 0
	require.Truef(hasSymlinkFlag, "target symlink does not have the symlink flag: %s", symlinkPath)

	actualPointsTo, err := ifs.Readlink(symlinkPath)
	require.NoError(err)

	require.True(actualPointsTo != "", "symlink target path is empty")
}

func SymlinkMustExistWithTragetPath(t *testing.T, ifs interfaces.Fs, symlinkPath, expectedPointsTo string) {
	symlinkPath = filepath.Clean(symlinkPath)
	expectedPointsTo = filepath.Clean(expectedPointsTo)

	require := require.New(t)

	fi, err := ifs.Lstat(symlinkPath)
	require.False(errors.Is(err, fs.ErrNotExist), "target symlink does not exist but is expected to exist: ", symlinkPath)
	require.NoError(err)

	hasSymlinkFlag := fi.Mode()&os.ModeType&os.ModeSymlink != 0
	require.True(hasSymlinkFlag, "target symlink does not have the symlink flag: ", symlinkPath)

	actualPointsTo, err := ifs.Readlink(symlinkPath)
	require.NoError(err)

	require.Equal(expectedPointsTo, actualPointsTo, "symlink does not point to the expected path")

}

func MustNotExist(t *testing.T, fs interfaces.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := fsutils.Exists(fs, path)
	require.NoError(err)
	require.False(found, "found file path but should not exist: "+path)
}

func MustNotLExist(t *testing.T, fs interfaces.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := fsutils.LExists(fs, path)
	require.NoError(err)
	require.Falsef(found, "path found but should not exist: %s", path)
}

func MustExist(t *testing.T, fs interfaces.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := fsutils.Exists(fs, path)
	require.NoError(err)
	require.True(found, "file path not found but should exist: "+path)
}

func MustLExist(t *testing.T, fs interfaces.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := fsutils.LExists(fs, path)
	require.NoError(err)
	require.Truef(found, "path not found but should exist: %s", path)
}

func RemoveFile(t *testing.T, fs interfaces.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)

	err := fs.Remove(path)
	require.NoError(err)

	MustNotLExist(t, fs, path)
}

func RemoveAll(t *testing.T, fs interfaces.Fs, path string) {
	path = filepath.Clean(path)

	require := require.New(t)

	err := fs.RemoveAll(path)
	require.NoError(err)

	MustNotLExist(t, fs, path)
}

func CreateSymlink(t *testing.T, ifs interfaces.Fs, oldpath, newpath string) {
	require := require.New(t)

	oldpath = filepath.Clean(oldpath)
	newpath = filepath.Clean(newpath)

	dirPath := filepath.Dir(oldpath)
	found, err := fsutils.Exists(ifs, dirPath)
	require.NoError(err)

	if !found {
		err = ifs.MkdirAll(dirPath, 0o755)
		require.NoError(err)
	}

	dirPath = filepath.Dir(newpath)
	found, err = fsutils.Exists(ifs, dirPath)
	require.NoError(err)

	if !found {
		err = ifs.MkdirAll(dirPath, 0o755)
		require.NoError(err)
	}

	// check newpath after creating the symlink
	err = ifs.Symlink(oldpath, newpath)
	require.NoError(err)

	fi, err := ifs.Lstat(newpath)
	require.NoError(err)

	hasSymlinkFlag := fi.Mode()&os.ModeType&os.ModeSymlink != 0
	require.True(hasSymlinkFlag, "the target(newpath) symlink does not have the symlink flag set: ", newpath)

	// check oldpath after creating the symlink
	fi, err = ifs.Lstat(oldpath)
	switch {
	case err == nil:

		hasSymlinkFlag = fi.Mode()&os.ModeType&os.ModeSymlink != 0
		require.Falsef(hasSymlinkFlag, "the source (oldpath) symlink does have the symlink flag set but is expected not to have it set: %s", oldpath)
	case errors.Is(err, fs.ErrNotExist):
		// broken symlink that points to an invalid location may be created
		return
	default:
		require.NoError(err)
	}
}

func CreateFile(t *testing.T, fs interfaces.Fs, path, content string) {
	path = filepath.Clean(path)

	require := require.New(t)

	dirPath := filepath.Dir(path)
	found, err := fsutils.Exists(fs, dirPath)
	require.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0o755)
		require.NoError(err)
	}

	f, err := fs.Create(path)
	require.NoError(err)
	defer func(file interfaces.File) {
		err := f.Close()
		require.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	require.NoError(err)
	require.Equal(ret, len(content))
}

func OpenFile(t *testing.T, fs interfaces.Fs, path, content string, perm os.FileMode) {
	path = filepath.Clean(path)

	require := require.New(t)

	dirPath := filepath.Dir(path)
	found, err := fsutils.Exists(fs, dirPath)
	require.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0o755)
		require.NoError(err)
	}

	f, err := fs.OpenFile(path, os.O_RDWR|os.O_TRUNC|os.O_CREATE, perm)
	require.NoError(err)
	defer func(file interfaces.File) {
		err := f.Close()
		require.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	require.NoError(err)
	require.Equal(ret, len(content))
}

func MkdirAll(t *testing.T, fs interfaces.Fs, path string, perm os.FileMode) {
	path = filepath.Clean(path)

	require := require.New(t)
	err := fs.MkdirAll(path, perm)
	require.NoError(err)

	err = fsutils.IterateDirTree(path, func(s string) error {
		exists, err := fsutils.Exists(fs, s)
		if err != nil {
			return err
		}
		require.True(exists, "path not found but is expected to exist: ", s)
		return nil
	})
	require.NoError(err)
}

func Mkdir(t *testing.T, fs interfaces.Fs, path string, perm os.FileMode) error {
	path = filepath.Clean(path)

	require := require.New(t)
	err := fs.Mkdir(path, perm)
	if err != nil {
		// assert that it is indeed a path error
		_, ok := err.(*os.PathError)
		require.True(ok)
		return err
	}

	b, err := fsutils.LExists(fs, path)
	require.NoError(err)
	require.True(b, "directory: ", path, "must exist after it has been created but does not.")
	return nil
}

func ModeMustBeEqual(t *testing.T, a, b os.FileMode) {
	require := require.New(t)

	a &= internal.ChmodBits
	b &= internal.ChmodBits

	require.Equalf(a, b, "expected: %0o got: %0o", a, b)
}

func Chmod(t *testing.T, ifs interfaces.Fs, path string, perm os.FileMode) {
	path = filepath.Clean(path)
	require := require.New(t)

	exists, err := fsutils.LExists(ifs, path)
	require.NoError(err)

	if !exists {
		err = ifs.Chmod(path, perm)
		require.Error(err)
		return
	}

	err = ifs.Chmod(path, perm)
	require.NoError(err)

	fiAfter, err := ifs.Lstat(path)
	require.NoError(err)

	permAfter := fiAfter.Mode()

	ModeMustBeEqual(t, perm, permAfter)
}

func CountFiles(t *testing.T, ifs interfaces.Fs, path string, expectedFilesAndDirs int) {
	require := require.New(t)
	path = filepath.Clean(path)

	files, err := fsutils.ListFiles(ifs, path)
	require.NoError(err)

	sort.Strings(files)

	require.Equalf(expectedFilesAndDirs, len(files), "files: %v", files)
}
