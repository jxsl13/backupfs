package internal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func CreateMemDir(path string, perm os.FileMode) os.FileInfo {
	fd := mem.CreateDir(path)
	mem.SetMode(fd, os.ModeDir|perm)
	return mem.GetFileInfo(fd)
}

func CreateMemFile(path, content string, perm os.FileMode) afero.File {
	fd := mem.CreateFile(path)
	mem.SetMode(fd, perm)
	return mem.NewFileHandle(fd)
}

func FileMustContainText(t *testing.T, fs afero.Fs, path, content string) {
	require := require.New(t)
	f, err := fs.Open(path)
	require.NoError(err)
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	require.NoError(err)

	require.Equal(string(b), content)
}

func MustNotExist(t *testing.T, fs afero.Fs, path string) {
	require := require.New(t)
	found, err := Exists(fs, path)
	require.NoError(err)
	require.False(found, "found file path but should not exist: "+path)
}

func MustExist(t *testing.T, fs afero.Fs, path string) {
	require := require.New(t)
	found, err := Exists(fs, path)
	require.NoError(err)
	require.True(found, "file path not found but should exist: "+path)
}

func RemoveFile(t *testing.T, fs afero.Fs, path string) {
	require := require.New(t)

	err := fs.Remove(path)
	require.NoError(err)
}

func RemoveAll(t *testing.T, fs afero.Fs, path string) {
	require := require.New(t)

	err := fs.RemoveAll(path)
	require.NoError(err)

	MustNotExist(t, fs, path)
}

func CreateFile(t *testing.T, fs afero.Fs, path, content string) {
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

var umaskVal = (*uint32)(nil)

func Umask() uint32 {
	if umaskVal == nil {
		umaskValue := uint32(unix.Umask(0))
		_ = unix.Umask(int(umaskValue))
		umaskVal = &umaskValue
	}
	return *umaskVal
}

func OpenFile(t *testing.T, fs afero.Fs, path, content string, perm os.FileMode) {
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
