package backupfs

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jxsl13/backupfs/interfaces"
	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/jxsl13/backupfs/mem"
	"github.com/jxsl13/backupfs/osfs"
	"github.com/stretchr/testify/require"
)

var (
	mm      interfaces.Fs
	mo      sync.Once
	resetMu sync.Mutex
)

func NewTestMemMapFs() interfaces.Fs {
	mo.Do(func() {
		resetMu.Lock()
		defer resetMu.Unlock()
		mm = mem.NewMemMapFs()
	})

	return mm
}

func ResetTestMemMapFs() {
	resetMu.Lock()
	defer resetMu.Unlock()
	mm = mem.NewMemMapFs()
}

func NewTestPrefixFs(prefix string) *PrefixFs {
	return NewPrefixFs(prefix, NewTestMemMapFs())
}

// this helper function is needed in order to test on the local filesystem
// and not in memory
func NewTempdirPrefixFs(prefix string) *PrefixFs {
	osFs := osfs.New()
	tmpDir := os.TempDir()
	err := os.MkdirAll(tmpDir, 0o700)
	if err != nil {
		log.Fatalln(err)
	}
	volume := filepath.VolumeName(tmpDir)
	volumeFs := NewVolumeFs(volume, osFs)
	tmpDir = TrimVolume(tmpDir)

	// remove volume from temp dir

	prefix, err = tempDir(volumeFs, tmpDir, prefix)
	if err != nil {
		log.Fatalln(err)
	}

	return NewPrefixFs(prefix, volumeFs)
}

func NewTestBackupFs(basePrefix, backupPrefix string) (root, base, backup interfaces.Fs, backupFs *BackupFs) {
	root = NewTestPrefixFs("/")
	base = NewTestPrefixFs(basePrefix)
	backup = NewTestPrefixFs(backupPrefix)
	backupFs = NewBackupFs(base, backup)
	return root, base, backup, backupFs
}

func NewTestTempdirBackupFs(basePrefix, backupPrefix string) (base, backup interfaces.Fs, backupFs *BackupFs) {

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
	testutils.CreateFile(t, base, filePath, fileContent)

	testutils.CreateFile(t, backupFs, filePath, fileContentOverwritten)

	testutils.FileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	testutils.FileMustContainText(t, root, "backup"+filePath, fileContent)

	testutils.CreateFile(t, backupFs, filePath, fileContentOverwrittenAgain)
	testutils.FileMustContainText(t, backupFs, filePath, fileContentOverwrittenAgain)
	testutils.FileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	testutils.FileMustContainText(t, root, "backup"+filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	testutils.CreateFile(t, backupFs, newFilePath, fileContent)
	testutils.FileMustContainText(t, root, "base"+newFilePath, fileContent)
	testutils.MustNotExist(t, root, "backup"+newFilePath)
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
	testutils.OpenFile(t, base, filePath, fileContent, 0o755)

	testutils.OpenFile(t, backupFs, filePath, fileContentOverwritten, 0o1755)

	testutils.FileMustContainText(t, root, "base"+filePath, fileContentOverwritten)
	testutils.FileMustContainText(t, root, "backup"+filePath, fileContent)

	testutils.OpenFile(t, backupFs, filePath, fileContentOverwrittenAgain, 0o766)
	testutils.FileMustContainText(t, backupFs, filePath, fileContentOverwrittenAgain)
	testutils.FileMustContainText(t, root, "base"+filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	testutils.FileMustContainText(t, root, "backup"+filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	testutils.OpenFile(t, backupFs, newFilePath, fileContent, 0o755)
	testutils.FileMustContainText(t, root, "base"+newFilePath, fileContent)
	testutils.MustNotExist(t, root, "backup"+newFilePath)
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
	testutils.CreateFile(t, base, filePath, fileContent)
	testutils.FileMustContainText(t, root, "base"+filePath, fileContent)

	testutils.RemoveFile(t, backupFs, filePath)
	testutils.MustNotExist(t, backupFs, filePath)

	testutils.MustNotExist(t, base, filePath)
	testutils.MustNotExist(t, root, "base"+filePath)

	testutils.MustExist(t, backup, filePath)
	testutils.MustExist(t, root, "backup"+filePath)
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

	testutils.MkdirAll(t, base, fileDir, 0o755)
	testutils.MkdirAll(t, base, fileDir2, 0o755)
	testutils.MkdirAll(t, base, symlinkDir, 0o755)

	testutils.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	testutils.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// symlink pointing at random location that doesnot exist
	testutils.CreateSymlink(t, base, fileDir+"/test00.txt", symlinkDir+"/link")
	testutils.CreateSymlink(t, base, fileDir+"/test00.txt", symlinkDir+"/link2")

	testutils.RemoveAll(t, backupFs, symlinkDir+"/link")
	testutils.MustNotLExist(t, backupFs, symlinkDir+"/link")

	// remove /test dir
	testutils.RemoveAll(t, backupFs, fileDirRoot)
	testutils.MustNotExist(t, backupFs, fileDirRoot)

	// deleted from base file system
	testutils.MustNotExist(t, base, fileDir+"/test01.txt")
	testutils.MustNotExist(t, base, fileDir+"/test02.txt")
	testutils.MustNotExist(t, base, fileDir2+"/test03.txt")
	testutils.MustNotExist(t, base, fileDir2+"/test04.txt")

	// link2 is a symlink in one of the sub folders in the
	// directory that is being removed with all of its content
	testutils.MustNotLExist(t, backupFs, symlinkDir+"/link2")

	testutils.MustNotExist(t, base, fileDirRoot)
	testutils.MustNotExist(t, base, fileDir)
	testutils.MustNotExist(t, base, fileDir2)

	// must exist in bakcup
	testutils.FileMustContainText(t, backup, fileDir+"/test01.txt", fileContent)
	testutils.FileMustContainText(t, backup, fileDir+"/test02.txt", fileContent)
	testutils.FileMustContainText(t, backup, fileDir2+"/test03.txt", fileContent)
	testutils.FileMustContainText(t, backup, fileDir2+"/test04.txt", fileContent)

	testutils.MustExist(t, backup, fileDir)
	testutils.MustExist(t, backup, fileDir2)

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

	err := base.MkdirAll(oldDirName, 0o755)
	require.NoError(err)
	testutils.MustExist(t, root, "base"+oldDirName)

	err = backupFs.Rename(oldDirName, newDirName)
	require.NoError(err)

	testutils.MustNotExist(t, backupFs, oldDirName)
	testutils.MustExist(t, backupFs, newDirName)

	testutils.MustNotExist(t, base, oldDirName)
	testutils.MustExist(t, base, newDirName)

	testutils.MustNotExist(t, backup, newDirName)
	testutils.MustExist(t, backup, oldDirName)

	err = backupFs.Rename(newDirName, newerDirName)
	require.NoError(err)

	testutils.MustNotExist(t, backupFs, newDirName)
	testutils.MustExist(t, backupFs, newerDirName)

	testutils.MustExist(t, backup, oldDirName)
	testutils.MustNotExist(t, backup, newDirName)
	testutils.MustNotExist(t, backup, newerDirName)
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

	testutils.MkdirAll(t, base, fileDir, 0o755)
	testutils.MkdirAll(t, base, fileDir2, 0o755)

	testutils.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	testutils.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// delete directory & files that did exist before
	testutils.RemoveAll(t, backupFs, fileDir)

	// removed files must not exist
	testutils.MustNotExist(t, base, fileDir)
	testutils.MustNotExist(t, base, fileDir+"/test01.txt")
	testutils.MustNotExist(t, base, fileDir+"/test02.txt")

	testutils.MustNotExist(t, backupFs, fileDir)
	testutils.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	testutils.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	testutils.MustExist(t, backup, fileDirRoot)
	testutils.MustExist(t, backup, fileDir)
	testutils.FileMustContainText(t, backup, fileDir+"/test01.txt", fileContent)
	testutils.FileMustContainText(t, backup, fileDir+"/test02.txt", fileContent)

	// create files that did not exist before
	testutils.CreateFile(t, backupFs, fileDir2+"/test05_new.txt", fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	testutils.MustNotExist(t, backup, fileDir2+"/test05_new.txt")

	// create subdir of deleted directory which did not exist before
	testutils.MkdirAll(t, backupFs, "/test/001/subdir_new", 0o755)
	testutils.CreateFile(t, backupFs, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	testutils.MustNotExist(t, backup, "/test/001/subdir_new")
	testutils.MustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err := backupFs.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	testutils.MustExist(t, backupFs, fileDir)
	testutils.MustExist(t, backupFs, fileDir+"/test01.txt")
	testutils.MustExist(t, backupFs, fileDir+"/test02.txt")

	// also restored in the underlying filesystem
	testutils.MustExist(t, base, fileDir)
	testutils.MustExist(t, base, fileDir+"/test01.txt")
	testutils.MustExist(t, base, fileDir+"/test02.txt")

	// newly created files must have been deleted upon rollback
	testutils.MustNotExist(t, base, fileDir2+"/test05_new.txt")
	testutils.MustNotExist(t, backupFs, fileDir2+"/test05_new.txt")

	// new files should have been deleted
	testutils.MustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	testutils.MustNotExist(t, backupFs, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	testutils.MustNotExist(t, base, "/test/001/subdir_new")
	testutils.MustNotExist(t, backupFs, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	testutils.MustExist(t, base, fileDir)
	testutils.MustExist(t, backupFs, fileDir)
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

	testutils.MkdirAll(t, base, fileDir, 0o755)
	testutils.MkdirAll(t, base, fileDir2, 0o755)

	testutils.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	testutils.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// delete directory & files that did exist before
	testutils.RemoveAll(t, backupFs, fileDir)

	// force  backup of a deleted directoty
	//  which existed before
	err := backupFs.ForceBackup(fileDir)
	require.NoError(err)

	// removed files must not exist
	testutils.MustNotExist(t, base, fileDir)
	testutils.MustNotExist(t, base, fileDir+"/test01.txt")
	testutils.MustNotExist(t, base, fileDir+"/test02.txt")

	testutils.MustNotExist(t, backupFs, fileDir)
	testutils.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	testutils.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	testutils.MustExist(t, backup, fileDirRoot)
	testutils.MustNotExist(t, backup, fileDir)
	testutils.MustNotExist(t, backup, fileDir+"/test01.txt")
	testutils.MustNotExist(t, backup, fileDir+"/test02.txt")

	// create files that did not exist before
	testutils.CreateFile(t, backupFs, fileDir2+"/test05_new.txt", fileContentNew)
	testutils.CreateFile(t, backupFs, fileDir2+"/test06_new.txt", fileContentNew)

	testutils.MustNotExist(t, backup, fileDir2+"/test05_new.txt")
	testutils.MustNotExist(t, backup, fileDir2+"/test06_new.txt")

	err = backupFs.ForceBackup(fileDir2 + "/test05_new.txt")
	require.NoError(err)

	testutils.FileMustContainText(t, backup, fileDir2+"/test05_new.txt", fileContentNew)

	testutils.MkdirAll(t, backupFs, "/test/001/subdir_new", 0o755)
	testutils.CreateFile(t, backupFs, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	testutils.MustNotExist(t, backup, "/test/001/subdir_new")
	testutils.MustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err = backupFs.Rollback()
	require.NoError(err)
	// ROLLBACK

	testutils.MustNotExist(t, backupFs, fileDir)
	testutils.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	testutils.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	testutils.MustNotExist(t, base, fileDir)
	testutils.MustNotExist(t, base, fileDir+"/test01.txt")
	testutils.MustNotExist(t, base, fileDir+"/test02.txt")

	testutils.MustExist(t, base, fileDir2+"/test05_new.txt")
	testutils.MustExist(t, backupFs, fileDir2+"/test05_new.txt")
	testutils.MustNotExist(t, base, fileDir2+"/test06_new.txt")
	testutils.MustNotExist(t, backupFs, fileDir2+"/test06_new.txt")

	testutils.MustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	testutils.MustNotExist(t, backupFs, "/test/001/subdir_new/test06_new.txt")

	testutils.MustNotExist(t, base, "/test/001/subdir_new")
	testutils.MustNotExist(t, backupFs, "/test/001/subdir_new")

	// we forced the deletion of the fileDir to be backed up
	// this means the the folder and its contents do not exist anymore
	testutils.MustNotExist(t, base, fileDir)
	testutils.MustNotExist(t, backupFs, fileDir)
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

	testutils.MkdirAll(t, base, fileDir, 0o755)
	testutils.MkdirAll(t, base, fileDir2, 0o755)

	testutils.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	testutils.CreateFile(t, base, fileDir+"/test02.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test03.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test04.txt", fileContent)

	// delete directory & files that did exist before
	testutils.RemoveAll(t, backupFs, fileDir)

	// removed files must not exist
	testutils.MustNotExist(t, base, fileDir)
	testutils.MustNotExist(t, base, fileDir+"/test01.txt")
	testutils.MustNotExist(t, base, fileDir+"/test02.txt")

	testutils.MustNotExist(t, backupFs, fileDir)
	testutils.MustNotExist(t, backupFs, fileDir+"/test01.txt")
	testutils.MustNotExist(t, backupFs, fileDir+"/test02.txt")

	testutils.MustExist(t, backup, fileDirRoot)
	testutils.MustExist(t, backup, fileDir)
	testutils.FileMustContainText(t, backup, fileDir+"/test01.txt", fileContent)
	testutils.FileMustContainText(t, backup, fileDir+"/test02.txt", fileContent)

	// create files that did not exist before
	testutils.CreateFile(t, backupFs, fileDir2+"/test05_new.txt", fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	testutils.MustNotExist(t, backup, fileDir2+"/test05_new.txt")

	// create subdir of deleted directory which did not exist before
	testutils.MkdirAll(t, backupFs, "/test/001/subdir_new", 0o755)
	testutils.CreateFile(t, backupFs, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	testutils.MustNotExist(t, backup, "/test/001/subdir_new")
	testutils.MustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

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
	testutils.MustExist(t, backupFsNew, fileDir)
	testutils.MustExist(t, backupFsNew, fileDir+"/test01.txt")
	testutils.MustExist(t, backupFsNew, fileDir+"/test02.txt")

	// also restored in the underlying filesystem
	testutils.MustExist(t, base, fileDir)
	testutils.MustExist(t, base, fileDir+"/test01.txt")
	testutils.MustExist(t, base, fileDir+"/test02.txt")

	// newly created files must have been deleted upon rollback
	testutils.MustNotExist(t, base, fileDir2+"/test05_new.txt")
	testutils.MustNotExist(t, backupFsNew, fileDir2+"/test05_new.txt")

	// new files should have been deleted
	testutils.MustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	testutils.MustNotExist(t, backupFsNew, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	testutils.MustNotExist(t, base, "/test/001/subdir_new")
	testutils.MustNotExist(t, backupFsNew, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	testutils.MustExist(t, base, "/test/001")
	testutils.MustExist(t, backupFsNew, "/test/001")

}

func TestBackupFs_Symlink(t *testing.T) {

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

	testutils.MkdirAll(t, base, fileDir, 0o755)
	testutils.MkdirAll(t, base, fileDir2, 0o755)

	testutils.CreateFile(t, base, fileDir+"/test01.txt", fileContent)
	testutils.CreateFile(t, base, fileDir2+"/test02.txt", fileContent)

	testutils.CreateSymlink(t, base, fileDir+"/test01.txt", fileDirRoot+"/file_symlink")
	testutils.CreateSymlink(t, base, fileDir, fileDirRoot+"/directory_symlink")

	// modify through backupFs layer

	// the old symlink must have been backed up after this call

	testutils.RemoveFile(t, backupFs, fileDirRoot+"/file_symlink")
	testutils.RemoveFile(t, backupFs, fileDirRoot+"/directory_symlink")

	// potential problem case:
	// Symlink creation fails midway due to another file, directory or symlink already existing.
	// due to the writing character of the symlink method we do create a backup
	// but fail to create a new symlink thus the backedup file and the old symlink are indeed the exact same
	// not exactly a problem but may caus eunnecessary backe dup data
	testutils.CreateSymlink(t, backupFs, fileDir2+"/test02.txt", fileDirRoot+"/file_symlink")

	testutils.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/file_symlink", fileDir2+"/test02.txt")

	testutils.SymlinkMustExistWithTragetPath(t, backup, fileDirRoot+"/file_symlink", fileDir+"/test01.txt")

	// create folder symlinks
	testutils.CreateSymlink(t, backupFs, fileDir2, fileDirRoot+"/directory_symlink")
	testutils.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/directory_symlink", fileDir2)
	testutils.SymlinkMustExistWithTragetPath(t, backup, fileDirRoot+"/directory_symlink", fileDir)

	testutils.CreateSymlink(t, backupFs, fileDir2+"/does_not_exist", "/to_be_removed_symlink")

	err := backupFs.Rollback()
	require.NoError(err)

	// assert both base symlinks point to their respective previous paths
	testutils.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/file_symlink", fileDir+"/test01.txt")
	testutils.SymlinkMustExistWithTragetPath(t, backupFs, fileDirRoot+"/directory_symlink", fileDir)

	testutils.SymlinkMustExistWithTragetPath(t, base, fileDirRoot+"/file_symlink", fileDir+"/test01.txt")
	testutils.SymlinkMustExistWithTragetPath(t, base, fileDirRoot+"/directory_symlink", fileDir)

	// never existed before, was created and then rolled back
	testutils.MustNotLExist(t, backupFs, "/to_be_removed_symlink")

	testutils.MustNotLExist(t, backup, fileDirRoot+"/file_symlink")
	testutils.MustNotLExist(t, backup, fileDirRoot+"/directory_symlink")

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

	err := testutils.Mkdir(t, base, fileDirRoot, 0o755)
	require.NoError(err)

	err = testutils.Mkdir(t, backupFs, fileDir2, 0o755)
	require.Error(err, "cannot create child directory without having created its parent")

	err = testutils.Mkdir(t, backupFs, fileDir, 0o755)
	require.NoError(err)

	err = testutils.Mkdir(t, backupFs, fileDir2, 0o755)
	require.NoError(err)

	testutils.RemoveAll(t, backupFs, fileDirRoot)

	// /test existed in the base filesystem and has been removed at the end -> upon removal we backup this directory.
	testutils.MustLExist(t, backup, fileDirRoot)
}

func TestBackupFs_Chmod(t *testing.T) {
	ResetTestMemMapFs()
	var (
		require      = require.New(t)
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	// MemmapFs does not seem to properly implement chmod stuff.
	base, backup, backupFs := NewTestTempdirBackupFs(basePrefix, backupPrefix)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		filePath    = fileDirRoot + "/test_file_chmod.txt"
	)
	testutils.CreateFile(t, base, filePath, "chmod test file")

	// get initial permission bits
	initialFi, err := base.Lstat(filePath)
	require.NoError(err)
	initialMode := initialFi.Mode()

	// change mod
	expectedNewPerm := os.FileMode(0644)
	testutils.Chmod(t, backupFs, filePath, expectedNewPerm)

	// get backed up file permissions
	fi, err := backup.Lstat(filePath)
	require.NoError(err)

	// compare backed up permissions to initial permissions
	backedUpPerm := fi.Mode()
	testutils.ModeMustBeEqual(t, initialMode, backedUpPerm)
}

func TestTime(t *testing.T) {
	require := require.New(t)

	t1 := time.Now()
	nanoBefore := t1.UnixNano()

	t2 := time.Unix(nanoBefore/1000000000, nanoBefore%1000000000)
	require.Equal(t1.UnixNano(), t2.UnixNano())
}
