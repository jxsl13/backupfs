package internal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func FileMustContainText(t *testing.T, fs afero.Fs, path, content string) {
	assert := assert.New(t)
	f, err := fs.Open(path)
	assert.NoError(err)
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	assert.NoError(err)

	assert.Equal(string(b), content)
}

func MustNotExist(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)
	found, err := Exists(fs, path)
	assert.NoError(err)
	assert.False(found, "found file path but should not exist: "+path)
}

func MustExist(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)
	found, err := Exists(fs, path)
	assert.NoError(err)
	assert.True(found, "found file path but should exist: "+path)
}

func RemoveFile(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)

	err := fs.Remove(path)
	assert.NoError(err)
}

func CreateFile(t *testing.T, fs afero.Fs, path, content string) {
	assert := assert.New(t)

	dirPath := filepath.Dir(path)
	found, err := Exists(fs, dirPath)
	assert.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0755)
		assert.NoError(err)
	}

	f, err := fs.Create(path)
	assert.NoError(err)
	defer func(file afero.File) {
		err := f.Close()
		assert.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	assert.NoError(err)
	assert.Equal(ret, len(content))
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
	assert := assert.New(t)

	dirPath := filepath.Dir(path)
	found, err := Exists(fs, dirPath)
	assert.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0755)
		assert.NoError(err)
	}

	f, err := fs.OpenFile(path, os.O_RDWR|os.O_TRUNC|os.O_CREATE, perm)
	assert.NoError(err)
	defer func(file afero.File) {
		err := f.Close()
		assert.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	assert.NoError(err)
	assert.Equal(ret, len(content))

}
