package backupfs

import (
	"sync"
	"testing"

	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var (
	mm      afero.Fs
	mo      sync.Once
	resetMu sync.Mutex
)

func NewTestMemMapFs() afero.Fs {
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
	internal.CreateFile(t, base, filePath, fileContent)

	internal.CreateFile(t, backupFs, filePath, fileContentOverwritten)

	internal.FileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	internal.FileMustContainText(t, root, "backup"+filePath, fileContent)

	internal.CreateFile(t, backupFs, filePath, fileContentOverwrittenAgain)
	internal.FileMustContainText(t, backupFs, filePath, fileContentOverwrittenAgain)
	internal.FileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	internal.FileMustContainText(t, root, "backup"+filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	internal.CreateFile(t, backupFs, newFilePath, fileContent)
	internal.FileMustContainText(t, root, "base"+newFilePath, fileContent)
	internal.MustNotExist(t, root, "backup"+newFilePath)
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
	internal.OpenFile(t, base, filePath, fileContent, 0755)

	internal.OpenFile(t, backupFs, filePath, fileContentOverwritten, 1755)

	internal.FileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	internal.FileMustContainText(t, root, "backup"+filePath, fileContent)

	internal.OpenFile(t, backupFs, filePath, fileContentOverwrittenAgain, 0766)
	internal.FileMustContainText(t, backupFs, filePath, fileContentOverwrittenAgain)
	internal.FileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	internal.FileMustContainText(t, root, "backup"+filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	internal.OpenFile(t, backupFs, newFilePath, fileContent, 0755)
	internal.FileMustContainText(t, root, "base"+newFilePath, fileContent)
	internal.MustNotExist(t, root, "backup"+newFilePath)
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
	internal.CreateFile(t, base, filePath, fileContent)
	internal.FileMustContainText(t, root, "base"+filePath, fileContent)

	internal.RemoveFile(t, backupFs, filePath)
	internal.MustNotExist(t, backupFs, filePath)

	internal.MustNotExist(t, base, filePath)
	internal.MustNotExist(t, root, "base"+filePath)

	internal.MustExist(t, backup, filePath)
	internal.MustExist(t, root, "backup"+filePath)
}

func TestBackupFsRemoveAll(t *testing.T) {
	ResetTestMemMapFs()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	_, base, backup, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/0/2"
		fileContent = "test_content"
	)

	internal.MkdirAll(t, base, fileDir, 0755)
	internal.MkdirAll(t, base, fileDir2, 0755)

	internal.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	internal.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	internal.RemoveAll(t, backupFs, fileDirRoot)

	// deleted from base file system
	internal.MustNotExist(t, base, fileDir+"/test01.txt")
	internal.MustNotExist(t, base, fileDir+"/test02.txt")
	internal.MustNotExist(t, base, fileDir2+"/test03.txt")
	internal.MustNotExist(t, base, fileDir2+"/test04.txt")

	internal.MustNotExist(t, base, fileDirRoot)
	internal.MustNotExist(t, base, fileDir)
	internal.MustNotExist(t, base, fileDir2)

	// must exist in bakcup
	internal.FileMustContainText(t, backup, fileDir+"/test01.txt", fileContent)
	internal.FileMustContainText(t, backup, fileDir+"/test02.txt", fileContent)
	internal.FileMustContainText(t, backup, fileDir2+"/test03.txt", fileContent)
	internal.FileMustContainText(t, backup, fileDir2+"/test04.txt", fileContent)

	internal.MustExist(t, backup, fileDir)
	internal.MustExist(t, backup, fileDir2)

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
	internal.MustExist(t, root, "base"+oldDirName)

	err = backupFs.Rename(oldDirName, newDirName)
	assert.NoError(err)

	internal.MustNotExist(t, backupFs, oldDirName)
	internal.MustExist(t, backupFs, newDirName)

	internal.MustNotExist(t, base, oldDirName)
	internal.MustExist(t, base, newDirName)

	internal.MustNotExist(t, backup, newDirName)
	internal.MustExist(t, backup, oldDirName)

	err = backupFs.Rename(newDirName, newerDirName)
	assert.NoError(err)

	internal.MustNotExist(t, backupFs, newDirName)
	internal.MustExist(t, backupFs, newerDirName)

	internal.MustExist(t, backup, oldDirName)
	internal.MustNotExist(t, backup, newDirName)
	internal.MustNotExist(t, backup, newerDirName)
}
