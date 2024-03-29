package backupfs

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

var (
	mm      afero.Fs
	mo      sync.Once
	resetMu sync.Mutex
)

// we need to extend the afero OsFs with our LchownIfPossible method for the tests.
type osFs struct {
	*afero.OsFs
}

func (fs osFs) LchownIfPossible(name string, uid, gid int) error {
	return os.Lchown(name, uid, gid)
}

func newOsFs() afero.Fs {
	return &osFs{
		&afero.OsFs{},
	}
}

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

// this helper function is needed in order to test on the local filesystem
// and not in memory
func NewTempdirPrefixFs(prefix string) *PrefixFs {
	osFs := newOsFs()

	tempDir := os.TempDir()
	err := os.MkdirAll(tempDir, 0700)
	if err != nil {
		log.Fatalln(err)
	}
	volume := filepath.VolumeName(tempDir)
	volumeFs := NewVolumeFs(volume, osFs)
	tempDir = TrimVolume(tempDir)

	// remove volume from temp dir

	prefix, err = afero.TempDir(volumeFs, tempDir, prefix)
	if err != nil {
		log.Fatalln(err)
	}

	return NewPrefixFs(prefix, volumeFs)
}

func NewTestBackupFs(basePrefix, backupPrefix string) (root, base, backup afero.Fs, backupFs *BackupFs) {
	root = NewTestPrefixFs("/")
	base = NewTestPrefixFs(basePrefix)
	backup = NewTestPrefixFs(backupPrefix)
	backupFs = NewBackupFs(base, backup)
	return root, base, backup, backupFs
}

func NewTestTempdirBackupFs(basePrefix, backupPrefix string) (base, backup afero.Fs, backupFs *BackupFs) {

	base = NewTempdirPrefixFs(basePrefix)
	backup = NewTempdirPrefixFs(backupPrefix)
	backupFs = NewBackupFs(base, backup)
	return base, backup, backupFs
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

	require := require.New(t)
	_, _, _, backupFs := NewTestBackupFs("/base", "/backup")

	require.Equal(backupFs.Name(), "BackupFs")
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
	base, backup, backupFs := NewTestTempdirBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/0/2"
		symlinkDir  = "/test/sym"
		fileContent = "test_content"
	)

	internal.MkdirAll(t, base, fileDir, 0755)
	internal.MkdirAll(t, base, fileDir2, 0755)
	internal.MkdirAll(t, base, symlinkDir, 0755)

	internal.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	internal.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// symlink pointing at random location that doesnot exist
	internal.CreateSymlink(t, base, fileDir+"/test00.txt", symlinkDir+"/link")
	internal.CreateSymlink(t, base, fileDir+"/test00.txt", symlinkDir+"/link2")

	internal.RemoveAll(t, backupFs, symlinkDir+"/link")
	internal.MustNotLExist(t, backupFs, symlinkDir+"/link")

	// remove /test dir
	internal.RemoveAll(t, backupFs, fileDirRoot)
	internal.MustNotExist(t, backupFs, fileDirRoot)

	// deleted from base file system
	internal.MustNotExist(t, base, fileDir+"/test01.txt")
	internal.MustNotExist(t, base, fileDir+"/test02.txt")
	internal.MustNotExist(t, base, fileDir2+"/test03.txt")
	internal.MustNotExist(t, base, fileDir2+"/test04.txt")

	// link2 is a symlink in one of the sub folders in the
	// directory that is being removed with all of its content
	internal.MustNotLExist(t, backupFs, symlinkDir+"/link2")

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
		require      = require.New(t)
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
	require.NoError(err)
	internal.MustExist(t, root, "base"+oldDirName)

	err = backupFs.Rename(oldDirName, newDirName)
	require.NoError(err)

	internal.MustNotExist(t, backupFs, oldDirName)
	internal.MustExist(t, backupFs, newDirName)

	internal.MustNotExist(t, base, oldDirName)
	internal.MustExist(t, base, newDirName)

	internal.MustNotExist(t, backup, newDirName)
	internal.MustExist(t, backup, oldDirName)

	err = backupFs.Rename(newDirName, newerDirName)
	require.NoError(err)

	internal.MustNotExist(t, backupFs, newDirName)
	internal.MustExist(t, backupFs, newerDirName)

	internal.MustExist(t, backup, oldDirName)
	internal.MustNotExist(t, backup, newDirName)
	internal.MustNotExist(t, backup, newerDirName)
}

func TestBackupFsRollback(t *testing.T) {
	ResetTestMemMapFs()

	var (
		require      = require.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, backup, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	internal.MkdirAll(t, base, fileDir, 0755)
	internal.MkdirAll(t, base, fileDir2, 0755)

	internal.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	internal.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// delete directory & files that did exist before
	internal.RemoveAll(t, backupFs, fileDir)

	// removed files must not exist
	internal.MustNotExist(t, base, fileDir)
	internal.MustNotExist(t, base, fileDir+"/test01.txt")
	internal.MustNotExist(t, base, fileDir+"/test02.txt")

	internal.MustNotExist(t, backupFs, fileDir)
	internal.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	internal.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	internal.MustExist(t, backup, fileDirRoot)
	internal.MustExist(t, backup, fileDir)
	internal.FileMustContainText(t, backup, fileDir+"/test01.txt", fileContent)
	internal.FileMustContainText(t, backup, fileDir+"/test02.txt", fileContent)

	// create files that did not exist before
	internal.CreateFile(t, backupFs, fileDir2+"/test05_new.txt", fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	internal.MustNotExist(t, backup, fileDir2+"/test05_new.txt")

	// create subdir of deleted directory which did not exist before
	internal.MkdirAll(t, backupFs, "/test/001/subdir_new", 0755)
	internal.CreateFile(t, backupFs, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	internal.MustNotExist(t, backup, "/test/001/subdir_new")
	internal.MustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err := backupFs.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	internal.MustExist(t, backupFs, fileDir)
	internal.MustExist(t, backupFs, fileDir+"/test01.txt")
	internal.MustExist(t, backupFs, fileDir+"/test02.txt")

	// also restored in the underlying filesystem
	internal.MustExist(t, base, fileDir)
	internal.MustExist(t, base, fileDir+"/test01.txt")
	internal.MustExist(t, base, fileDir+"/test02.txt")

	// newly created files must have been deleted upon rollback
	internal.MustNotExist(t, base, fileDir2+"/test05_new.txt")
	internal.MustNotExist(t, backupFs, fileDir2+"/test05_new.txt")

	// new files should have been deleted
	internal.MustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	internal.MustNotExist(t, backupFs, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	internal.MustNotExist(t, base, "/test/001/subdir_new")
	internal.MustNotExist(t, backupFs, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	internal.MustExist(t, base, fileDir)
	internal.MustExist(t, backupFs, fileDir)
}

func TestBackupFsRollbackWithForcedBackup(t *testing.T) {
	ResetTestMemMapFs()

	var (
		require      = require.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, backup, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	internal.MkdirAll(t, base, fileDir, 0755)
	internal.MkdirAll(t, base, fileDir2, 0755)

	internal.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	internal.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// delete directory & files that did exist before
	internal.RemoveAll(t, backupFs, fileDir)

	// force  backup of a deleted directoty
	//  which existed before
	err := backupFs.ForceBackup(fileDir)
	require.NoError(err)

	// removed files must not exist
	internal.MustNotExist(t, base, fileDir)
	internal.MustNotExist(t, base, fileDir+"/test01.txt")
	internal.MustNotExist(t, base, fileDir+"/test02.txt")

	internal.MustNotExist(t, backupFs, fileDir)
	internal.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	internal.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	internal.MustExist(t, backup, fileDirRoot)
	internal.MustNotExist(t, backup, fileDir)
	internal.MustNotExist(t, backup, fileDir+"/test01.txt")
	internal.MustNotExist(t, backup, fileDir+"/test02.txt")

	// create files that did not exist before
	internal.CreateFile(t, backupFs, fileDir2+"/test05_new.txt", fileContentNew)
	internal.CreateFile(t, backupFs, fileDir2+"/test06_new.txt", fileContentNew)

	internal.MustNotExist(t, backup, fileDir2+"/test05_new.txt")
	internal.MustNotExist(t, backup, fileDir2+"/test06_new.txt")

	err = backupFs.ForceBackup(fileDir2 + "/test05_new.txt")
	require.NoError(err)

	internal.FileMustContainText(t, backup, fileDir2+"/test05_new.txt", fileContentNew)

	internal.MkdirAll(t, backupFs, "/test/001/subdir_new", 0755)
	internal.CreateFile(t, backupFs, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	internal.MustNotExist(t, backup, "/test/001/subdir_new")
	internal.MustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err = backupFs.Rollback()
	require.NoError(err)
	// ROLLBACK

	internal.MustNotExist(t, backupFs, fileDir)
	internal.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	internal.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	internal.MustNotExist(t, base, fileDir)
	internal.MustNotExist(t, base, fileDir+"/test01.txt")
	internal.MustNotExist(t, base, fileDir+"/test02.txt")

	internal.MustExist(t, base, fileDir2+"/test05_new.txt")
	internal.MustExist(t, backupFs, fileDir2+"/test05_new.txt")
	internal.MustNotExist(t, base, fileDir2+"/test06_new.txt")
	internal.MustNotExist(t, backupFs, fileDir2+"/test06_new.txt")

	internal.MustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	internal.MustNotExist(t, backupFs, "/test/001/subdir_new/test06_new.txt")

	internal.MustNotExist(t, base, "/test/001/subdir_new")
	internal.MustNotExist(t, backupFs, "/test/001/subdir_new")

	// we forced the deletion of the fileDir to be backed up
	// this means the the folder and its contents do not exist anymore
	internal.MustNotExist(t, base, fileDir)
	internal.MustNotExist(t, backupFs, fileDir)
}

func TestBackupFsJSON(t *testing.T) {
	ResetTestMemMapFs()

	var (
		require      = require.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, backup, backupFs := NewTestBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	internal.MkdirAll(t, base, fileDir, 0755)
	internal.MkdirAll(t, base, fileDir2, 0755)

	internal.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	internal.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// delete directory & files that did exist before
	internal.RemoveAll(t, backupFs, fileDir)

	// removed files must not exist
	internal.MustNotExist(t, base, fileDir)
	internal.MustNotExist(t, base, fileDir+"/test01.txt")
	internal.MustNotExist(t, base, fileDir+"/test02.txt")

	internal.MustNotExist(t, backupFs, fileDir)
	internal.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	internal.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	internal.MustExist(t, backup, fileDirRoot)
	internal.MustExist(t, backup, fileDir)
	internal.FileMustContainText(t, backup, fileDir+"/test01.txt", fileContent)
	internal.FileMustContainText(t, backup, fileDir+"/test02.txt", fileContent)

	// create files that did not exist before
	internal.CreateFile(t, backupFs, fileDir2+"/test05_new.txt", fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	internal.MustNotExist(t, backup, fileDir2+"/test05_new.txt")

	// create subdir of deleted directory which did not exist before
	internal.MkdirAll(t, backupFs, "/test/001/subdir_new", 0755)
	internal.CreateFile(t, backupFs, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	internal.MustNotExist(t, backup, "/test/001/subdir_new")
	internal.MustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// JSON
	// after unmarshalling we should have the exact same behavior as without the marshaling/unmarshaling
	data, err := json.Marshal(backupFs)
	require.NoError(err)

	var backupFsNew *BackupFs = NewBackupFs(base, backup)
	err = json.Unmarshal(data, &backupFsNew)
	require.NoError(err)

	// JSON
	oldMap := backupFs.baseInfos
	newMap := backupFsNew.baseInfos

	for path, info := range oldMap {
		newInfo := newMap[path]

		if info == nil {
			require.Nil(newInfo)
			continue
		}

		require.Equal(info.IsDir(), newInfo.IsDir())
		require.Equal(info.Name(), newInfo.Name())
		require.Equal(info.Size(), newInfo.Size())
		require.Equal(info.ModTime().UnixNano(), newInfo.ModTime().UnixNano())
		require.Equal(info.Mode(), newInfo.Mode())
	}

	// ROLLBACK
	err = backupFs.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	internal.MustExist(t, backupFsNew, fileDir)
	internal.MustExist(t, backupFsNew, fileDir+"/test01.txt")
	internal.MustExist(t, backupFsNew, fileDir+"/test02.txt")

	// also restored in the underlying filesystem
	internal.MustExist(t, base, fileDir)
	internal.MustExist(t, base, fileDir+"/test01.txt")
	internal.MustExist(t, base, fileDir+"/test02.txt")

	// newly created files must have been deleted upon rollback
	internal.MustNotExist(t, base, fileDir2+"/test05_new.txt")
	internal.MustNotExist(t, backupFsNew, fileDir2+"/test05_new.txt")

	// new files should have been deleted
	internal.MustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	internal.MustNotExist(t, backupFsNew, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	internal.MustNotExist(t, base, "/test/001/subdir_new")
	internal.MustNotExist(t, backupFsNew, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	internal.MustExist(t, base, "/test/001")
	internal.MustExist(t, backupFsNew, "/test/001")

}

func TestBackupFs_SymlinkIfPossible(t *testing.T) {

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	base, backup, backupFs := NewTestTempdirBackupFs(basePrefix, backupPrefix)

	var (
		require = require.New(t)
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/0/2"
		fileContent = "test_content"
	)

	// base filesystem structure and files befor emodifying

	internal.MkdirAll(t, base, fileDir, 0755)
	internal.MkdirAll(t, base, fileDir2, 0755)

	internal.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	internal.CreateFile(t, base, fileDir2+"/test02.txt", fileContent)

	internal.CreateSymlink(t, base, fileDir+"/test01.txt", fileDirRoot+"/file_symlink")
	internal.CreateSymlink(t, base, fileDir, fileDirRoot+"/directory_symlink")

	// modify through backupFs layer

	// the old symlink must have been backed up after this call

	internal.RemoveFile(t, backupFs, fileDirRoot+"/file_symlink")
	internal.RemoveFile(t, backupFs, fileDirRoot+"/directory_symlink")

	// potential problem case:
	// Symlink creation fails midway due to another file, directory or symlink already existing.
	// due to the writing character of the symlink method we do create a backup
	// but fail to create a new symlink thus the backedup file and the old symlink are indeed the exact same
	// not exactly a problem but may caus eunnecessary backe dup data
	internal.CreateSymlink(t, backupFs, fileDir2+"/test02.txt", fileDirRoot+"/file_symlink")

	internal.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/file_symlink", fileDir2+"/test02.txt")

	internal.SymlinkMustExistWithTragetPath(t, backup, fileDirRoot+"/file_symlink", fileDir+"/test01.txt")

	// create folder symlinks
	internal.CreateSymlink(t, backupFs, fileDir2, fileDirRoot+"/directory_symlink")
	internal.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/directory_symlink", fileDir2)
	internal.SymlinkMustExistWithTragetPath(t, backup, fileDirRoot+"/directory_symlink", fileDir)

	internal.CreateSymlink(t, backupFs, fileDir2+"/does_not_exist", "/to_be_removed_symlink")

	err := backupFs.Rollback()
	require.NoError(err)

	// assert both base symlinks point to their respective previous paths
	internal.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/file_symlink", fileDir+"/test01.txt")
	internal.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/directory_symlink", fileDir)

	internal.SymlinkMustExistWithTragetPath(t, base, fileDirRoot+"/file_symlink", fileDir+"/test01.txt")
	internal.SymlinkMustExistWithTragetPath(t, base, fileDirRoot+"/directory_symlink", fileDir)

	// never existed before, was created and then rolled back
	internal.MustNotLExist(t, backupFs, "/to_be_removed_symlink")

	internal.MustNotLExist(t, backup, fileDirRoot+"/file_symlink")
	internal.MustNotLExist(t, backup, fileDirRoot+"/directory_symlink")

}

func TestBackupFs_Mkdir(t *testing.T) {

	var (
		require      = require.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	base, backup, backupFs := NewTestTempdirBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/001/002"
	)

	err := internal.Mkdir(t, base, fileDirRoot, 0755)
	require.NoError(err)

	err = internal.Mkdir(t, backupFs, fileDir2, 0755)
	require.Error(err, "cannot create child directory without having created its parent")

	err = internal.Mkdir(t, backupFs, fileDir, 0755)
	require.NoError(err)

	err = internal.Mkdir(t, backupFs, fileDir2, 0755)
	require.NoError(err)

	internal.RemoveAll(t, backupFs, fileDirRoot)

	// /test existed in the base filesystem and has been removed at the end -> upon removal we backup this directory.
	internal.MustLExist(t, backup, fileDirRoot)
}

func TestBackupFs_Chmod(t *testing.T) {
	ResetTestMemMapFs()
	var (
		require      = require.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	// Afero.MemmapFs does not seem to properly implement chmod stuff.
	base, backup, backupFs := NewTestTempdirBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		filePath    = fileDirRoot + "/test_file_chmod.txt"
	)
	internal.CreateFile(t, base, filePath, "chmod test file")

	// get initial permission bits
	initialFi, _, err := internal.LstatIfPossible(base, filePath)
	require.NoError(err)
	initialMode := initialFi.Mode()

	// change mod
	expectedNewPerm := os.FileMode(0644)
	internal.Chmod(t, backupFs, filePath, expectedNewPerm)

	// get backed up file permissions
	fi, _, err := internal.LstatIfPossible(backup, filePath)
	require.NoError(err)

	// compare backed up permissions to initial permissions
	backedUpPerm := fi.Mode()
	internal.ModeMustBeEqual(t, initialMode, backedUpPerm)
}

func TestTime(t *testing.T) {
	require := require.New(t)

	t1 := time.Now()
	nanoBefore := t1.UnixNano()

	t2 := time.Unix(nanoBefore/1000000000, nanoBefore%1000000000)
	require.Equal(t1.UnixNano(), t2.UnixNano())
}
