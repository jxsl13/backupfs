package backupfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var (
	mm      afero.Fs
	mo      sync.Once
	resetMu sync.Mutex
)

func NewTestMemMapFs() afero.Fs {
	if mm != nil {
		return mm
	}

	mo.Do(func() {
		resetMu.Lock()
		defer resetMu.Unlock()
		mm = afero.NewMemMapFs()
	})

	return mm
}

func ResetTestMemMapFs() {
	resetMu.Lock()
	defer resetMu.Unlock()
	mm = afero.NewMemMapFs()
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

func mustNotExist(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)
	found, err := exists(fs, path)
	assert.NoError(err)
	assert.False(found, "found file path but should not exist: "+path)
}

func mustExist(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)
	found, err := exists(fs, path)
	assert.NoError(err)
	assert.True(found, "found file path but should exist: "+path)
}

func removeFile(t *testing.T, fs afero.Fs, path string) {
	assert := assert.New(t)

	err := fs.Remove(path)
	assert.NoError(err)
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
	defer func(file afero.File) {
		err := f.Close()
		assert.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	assert.NoError(err)
	assert.Equal(ret, len(content))
}

var umaskVal = (*uint32)(nil)

func umask() uint32 {
	if umaskVal == nil {
		umaskValue := uint32(unix.Umask(0))
		_ = unix.Umask(int(umaskValue))
		umaskVal = &umaskValue
	}
	return *umaskVal
}

func openFile(t *testing.T, fs afero.Fs, path, content string, perm os.FileMode) {
	assert := assert.New(t)

	dirPath := filepath.Dir(path)
	found, err := exists(fs, dirPath)
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

func TestBackupFsCreate(t *testing.T) {
	ResetTestMemMapFs()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	root, base, _, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_content"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)
	createFile(t, base, filePath, fileContent)

	createFile(t, backupFs, filePath, fileContentOverwritten)

	fileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	fileMustContainText(t, root, "backup"+filePath, fileContent)

	createFile(t, backupFs, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, backupFs, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	fileMustContainText(t, root, "backup"+filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	createFile(t, backupFs, newFilePath, fileContent)
	fileMustContainText(t, root, "base"+newFilePath, fileContent)
	mustNotExist(t, root, "backup"+newFilePath)
}

func TestBackupFsName(t *testing.T) {
	ResetTestMemMapFs()

	assert := assert.New(t)
	_, _, _, backupFs := NewTestBackupFs("/base", "/backup")

	assert.Equal(backupFs.Name(), "BackupFs")
}

func TestBackupFsOpenFile(t *testing.T) {
	ResetTestMemMapFs()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	root, base, _, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_content"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)
	openFile(t, base, filePath, fileContent, 0755)

	openFile(t, backupFs, filePath, fileContentOverwritten, 1755)

	fileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	fileMustContainText(t, root, "backup"+filePath, fileContent)

	openFile(t, backupFs, filePath, fileContentOverwrittenAgain, 0766)
	fileMustContainText(t, backupFs, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	fileMustContainText(t, root, "backup"+filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	openFile(t, backupFs, newFilePath, fileContent, 0755)
	fileMustContainText(t, root, "base"+newFilePath, fileContent)
	mustNotExist(t, root, "backup"+newFilePath)
}

func TestBackupFsRemove(t *testing.T) {
	ResetTestMemMapFs()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	root, base, backup, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		filePath    = "/test/01/test_01.txt"
		fileContent = "test_content"
	)
	createFile(t, base, filePath, fileContent)
	fileMustContainText(t, root, "base"+filePath, fileContent)

	removeFile(t, backupFs, filePath)
	mustNotExist(t, backupFs, filePath)

	mustNotExist(t, base, filePath)
	mustNotExist(t, root, "base"+filePath)

	mustExist(t, backup, filePath)
	mustExist(t, root, "backup"+filePath)
}

func TestBackupFsRename(t *testing.T) {
	ResetTestMemMapFs()

	var (
		assert       = assert.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	root, base, backup, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		oldDirName   = "/test/rename"
		newDirName   = "/test/rename2"
		newerDirName = "/test/rename3"
	)

	err := base.MkdirAll(oldDirName, 0755)
	assert.NoError(err)
	mustExist(t, root, "base"+oldDirName)

	err = backupFs.Rename(oldDirName, newDirName)
	assert.NoError(err)

	mustNotExist(t, backupFs, oldDirName)
	mustExist(t, backupFs, newDirName)

	mustNotExist(t, base, oldDirName)
	mustExist(t, base, newDirName)

	mustNotExist(t, backup, newDirName)
	mustExist(t, backup, oldDirName)

	err = backupFs.Rename(newDirName, newerDirName)
	assert.NoError(err)

	mustNotExist(t, backupFs, newDirName)
	mustExist(t, backupFs, newerDirName)

	mustExist(t, backup, oldDirName)
	mustNotExist(t, backup, newDirName)
	mustNotExist(t, backup, newerDirName)
}
