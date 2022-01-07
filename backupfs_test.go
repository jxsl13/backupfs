package backupfs

import (
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var (
	mm afero.Fs
	mo sync.Once
)

func NewTestMemMapFs() afero.Fs {
	if mm != nil {
		return mm
	}

	mo.Do(func() {
		mm = afero.NewMemMapFs()
	})

	return mm
}

func NewTestPrefixFs(prefix string) *PrefixFs {
	return NewPrefixFs(prefix, NewTestMemMapFs())
}

func NewTestBackupFs(basePrefix, backupPrefix string) (root, base, backup, backupFs afero.Fs) {
	root = NewTestPrefixFs("/")
	base = NewTestPrefixFs(basePrefix)
	backup = NewTestPrefixFs(backupPrefix)
	backupFs = NewBackupFs(base, backup)
	return root, base, backup, backupFs
}

func fileMustContainText(t *testing.T, fs afero.Fs, path, content string) {
	assert := assert.New(t)
	f, err := fs.Open(path)
	assert.NoError(err)
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	assert.NoError(err)

	assert.Equal(string(b), content)
}

func fileMustNotExist(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)
	found, err := exists(fs, path)
	assert.NoError(err)
	assert.False(found, "found file path but should not exist: "+path)
}



func createFile(t *testing.T, fs afero.Fs, path, content string) {
	assert := assert.New(t)

	dirPath := filepath.Dir(path)
	found, err := exists(fs, dirPath)
	assert.NoError(err)

	if !found {
		err = fs.MkdirAll(dirPath, 0755)
		assert.NoError(err)
	}

	f, err := fs.Create(path)
	assert.NoError(err)
	defer func() {
		err := f.Close()
		assert.NoError(err)
	}()
	ret, err := f.WriteString(content)
	assert.NoError(err)
	assert.Equal(ret, len(content))
}

func TestBackupFsCreate(t *testing.T) {
	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	root, base, _, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_01"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)
	createFile(t, base, filePath, fileContent)

	createFile(t, backupFs, filePath, fileContentOverwritten)

	fileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	fileMustContainText(t, root, "backup"+filePath, fileContent)

	createFile(t, backupFs, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the sam estate as the first initial file
	fileMustContainText(t, root, "backup"+filePath, fileContent)
}
