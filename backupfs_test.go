package backupfs

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestBackupFS_Create(t *testing.T) {
	t.Parallel()

	root, base, backup, backupFS := NewTestBackupFS()
	defer func() {
		require.NoError(t, root.RemoveAll("/"))
	}()

	var (
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_content"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)
	createFile(t, base, filePath, fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	createFile(t, backupFS, filePath, fileContentOverwritten)

	fileMustContainText(t, base, filePath, fileContentOverwritten)
	fileMustContainText(t, backup, filePath, fileContent)

	createFile(t, backupFS, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, backupFS, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, base, filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	fileMustContainText(t, backup, filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	createFile(t, backupFS, newFilePath, fileContent)
	fileMustContainText(t, base, newFilePath, fileContent)
	mustNotExist(t, backup, newFilePath)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Name(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	_, _, _, backupFS := NewTestBackupFS()

	require.Equal(backupFS.Name(), "BackupFS")
}

func TestBackupFS_OpenFile(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_content"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)
	openFile(t, base, filePath, fileContent, 0755)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	openFile(t, backupFS, filePath, fileContentOverwritten, 1755)

	fileMustContainText(t, base, filePath, fileContentOverwritten)
	fileMustContainText(t, backup, filePath, fileContent)

	openFile(t, backupFS, filePath, fileContentOverwrittenAgain, 0766)
	fileMustContainText(t, backupFS, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, base, filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	fileMustContainText(t, backup, filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	openFile(t, backupFS, newFilePath, fileContent, 0755)
	fileMustContainText(t, base, newFilePath, fileContent)
	mustNotExist(t, backup, newFilePath)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Remove(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		filePath    = "/test/01/test_01.txt"
		fileContent = "test_content"
	)
	createFile(t, base, filePath, fileContent)
	fileMustContainText(t, base, filePath, fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	removeFile(t, backupFS, filePath)
	mustNotExist(t, backupFS, filePath)

	mustNotExist(t, base, filePath)

	mustExist(t, backup, filePath)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_RemoveAll(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/0/2"
		symlinkDir  = "/test/sym"
		fileContent = "test_content"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)
	mkdirAll(t, base, symlinkDir, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	// symlink pointing at random location that doesnot exist
	createSymlink(t, base, filepath.Join(fileDir, "/test00.txt"), filepath.Join(symlinkDir, "/link"))
	createSymlink(t, base, filepath.Join(fileDir, "/test00.txt"), filepath.Join(symlinkDir, "/link2"))

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	removeAll(t, backupFS, filepath.Join(symlinkDir, "/link"))
	removeAll(t, backupFS, filepath.Join(symlinkDir, "/fileDoesNotExistAndShouldNotThrowAnErrorWhenUsedInRemoveAll"))
	mustNotLExist(t, backupFS, filepath.Join(symlinkDir, "/link"))

	// remove /test dir
	removeAll(t, backupFS, fileDirRoot)
	mustNotExist(t, backupFS, fileDirRoot)

	// deleted from base file system
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))
	mustNotExist(t, base, filepath.Join(fileDir2, "/test03.txt"))
	mustNotExist(t, base, filepath.Join(fileDir2, "/test04.txt"))

	// link2 is a symlink in one of the sub folders in the
	// directory that is being removed with all of its content
	mustNotLExist(t, backupFS, filepath.Join(symlinkDir, "/link2"))

	mustNotExist(t, base, fileDirRoot)
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, fileDir2)

	// must exist in bakcup
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test01.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test02.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	mustExist(t, backup, fileDir)
	mustExist(t, backup, fileDir2)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Rename(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)
	_, base, backup, backupFS := NewTestBackupFS()

	var (
		oldDirName   = "/test/rename"
		newDirName   = "/test/rename2"
		newerDirName = "/test/rename3"
	)

	err := base.MkdirAll(oldDirName, 0755)
	require.NoError(err)
	mustExist(t, base, oldDirName)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	err = backupFS.Rename(oldDirName, newDirName)
	require.NoError(err)

	mustNotExist(t, backupFS, oldDirName)
	mustExist(t, backupFS, newDirName)

	mustNotExist(t, base, oldDirName)
	mustExist(t, base, newDirName)

	mustNotExist(t, backup, newDirName)
	mustExist(t, backup, oldDirName)

	err = backupFS.Rename(newDirName, newerDirName)
	require.NoError(err)

	mustNotExist(t, backupFS, newDirName)
	mustExist(t, backupFS, newerDirName)

	mustExist(t, backup, oldDirName)
	mustNotExist(t, backup, newDirName)
	mustNotExist(t, backup, newerDirName)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Rollback(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// delete directory & files that did exist before
	removeAll(t, backupFS, fileDir)

	// removed files must not exist
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, backup, fileDirRoot)
	mustExist(t, backup, fileDir)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test01.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test02.txt"), fileContent)

	// create files that did not exist before
	createFile(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	mustNotExist(t, backup, filepath.Join(fileDir2, "/test05_new.txt"))

	// create subdir of deleted directory which did not exist before
	mkdirAll(t, backupFS, "/test/001/subdir_new", 0755)
	createFile(t, backupFS, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	mustNotExist(t, backup, "/test/001/subdir_new")
	mustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	mustExist(t, backupFS, fileDir)
	mustExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	// also restored in the underlying filesystem
	mustExist(t, base, fileDir)
	mustExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	// newly created files must have been deleted upon rollback
	mustNotExist(t, base, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"))

	// new files should have been deleted
	mustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	mustNotExist(t, backupFS, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	mustNotExist(t, base, "/test/001/subdir_new")
	mustNotExist(t, backupFS, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	mustExist(t, base, fileDir)
	mustExist(t, backupFS, fileDir)

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_RollbackWithForcedBackup(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	// delete directory & files that did exist before
	removeAll(t, backupFS, fileDir)

	// force  backup of a deleted directoty
	//  which existed before
	err := backupFS.ForceBackup(fileDir)
	require.NoError(err)

	// removed files must not exist
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, backup, fileDirRoot)
	mustNotExist(t, backup, fileDir)
	mustNotExist(t, backup, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backup, filepath.Join(fileDir, "/test02.txt"))

	// create files that did not exist before
	createFile(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)
	createFile(t, backupFS, filepath.Join(fileDir2, "/test06_new.txt"), fileContentNew)

	mustNotExist(t, backup, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, backup, filepath.Join(fileDir2, "/test06_new.txt"))

	err = backupFS.ForceBackup(filepath.Join(fileDir2, "/test05_new.txt"))
	require.NoError(err)

	fileMustContainText(t, backup, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)

	mkdirAll(t, backupFS, "/test/001/subdir_new", 0755)
	createFile(t, backupFS, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	mustNotExist(t, backup, "/test/001/subdir_new")
	mustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, base, filepath.Join(fileDir2, "/test05_new.txt"))
	mustExist(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, base, filepath.Join(fileDir2, "/test06_new.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir2, "/test06_new.txt"))

	mustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	mustNotExist(t, backupFS, "/test/001/subdir_new/test06_new.txt")

	mustNotExist(t, base, "/test/001/subdir_new")
	mustNotExist(t, backupFS, "/test/001/subdir_new")

	// we forced the deletion of the fileDir to be backed up
	// this means the the folder and its contents do not exist anymore
	mustNotExist(t, base, fileDir)
	mustNotExist(t, backupFS, fileDir)
}

func TestBackupFS_JSON(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// delete directory & files that did exist before
	removeAll(t, backupFS, fileDir)

	// removed files must not exist
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, backup, fileDirRoot)
	mustExist(t, backup, fileDir)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test01.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test02.txt"), fileContent)

	// create files that did not exist before
	createFile(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	mustNotExist(t, backup, filepath.Join(fileDir2, "/test05_new.txt"))

	// create subdir of deleted directory which did not exist before
	mkdirAll(t, backupFS, "/test/001/subdir_new", 0755)
	createFile(t, backupFS, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	mustNotExist(t, backup, "/test/001/subdir_new")
	mustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// JSON
	// after unmarshalling we should have the exact same behavior as without the marshaling/unmarshaling
	data, err := json.Marshal(backupFS)
	require.NoError(err)

	var backupFSNew *BackupFS = NewBackupFS(base, backup)
	err = json.Unmarshal(data, &backupFSNew)
	require.NoError(err)

	// JSON
	oldMap := backupFS.baseInfos
	newMap := backupFSNew.baseInfos

	for path, info := range oldMap {
		newInfo := newMap[path]

		if info == nil {
			require.Nil(newInfo)
			continue
		}

		require.Equal(info.IsDir(), newInfo.IsDir())
		require.Equal(info.Name(), filepath.FromSlash(newInfo.Name()))
		require.Equal(info.Size(), newInfo.Size())
		require.Equal(info.ModTime().UnixNano(), newInfo.ModTime().UnixNano())
		require.Equal(info.Mode(), newInfo.Mode())

		if runtime.GOOS != "windows" {
			require.Greater(toUID(info), -1)
			require.Greater(toGID(info), -1)

			require.Greater(toUID(newInfo), -1)
			require.Greater(toGID(newInfo), -1)
		}
	}

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	mustExist(t, backupFSNew, fileDir)
	mustExist(t, backupFSNew, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, backupFSNew, filepath.Join(fileDir, "/test02.txt"))

	// also restored in the underlying filesystem
	mustExist(t, base, fileDir)
	mustExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	// newly created files must have been deleted upon rollback
	mustNotExist(t, base, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, backupFSNew, filepath.Join(fileDir2, "/test05_new.txt"))

	// new files should have been deleted
	mustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	mustNotExist(t, backupFSNew, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	mustNotExist(t, base, "/test/001/subdir_new")
	mustNotExist(t, backupFSNew, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	mustExist(t, base, "/test/001")
	mustExist(t, backupFSNew, "/test/001")

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Symlink(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

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

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test02.txt"), fileContent)

	createSymlink(t, base, filepath.Join(fileDir, "/test01.txt"), filepath.Join(fileDirRoot, "/file_symlink"))
	createSymlink(t, base, fileDir, filepath.Join(fileDirRoot, "/directory_symlink"))

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// modify through BackupFS layer

	// the old symlink must have been backed up after this call

	removeFile(t, backupFS, filepath.Join(fileDirRoot, "/file_symlink"))
	removeFile(t, backupFS, filepath.Join(fileDirRoot, "/directory_symlink"))

	// potential problem case:
	// Symlink creation fails midway due to another file, directory or symlink already existing.
	// due to the writing character of the symlink method we do create a backup
	// but fail to create a new symlink thus the backedup file and the old symlink are indeed the exact same
	// not exactly a problem but may caus eunnecessary backe dup data
	createSymlink(t, backupFS, filepath.Join(fileDir2, "/test02.txt"), filepath.Join(fileDirRoot, "/file_symlink"))

	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir2, "/test02.txt"))

	symlinkMustExistWithTragetPath(t, backup, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir, "/test01.txt"))

	// create folder symlinks
	createSymlink(t, backupFS, fileDir2, filepath.Join(fileDirRoot, "/directory_symlink"))
	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir2)
	symlinkMustExistWithTragetPath(t, backup, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir)

	createSymlink(t, backupFS, filepath.Join(fileDir2, "/does_not_exist"), "/to_be_removed_symlink")

	err := backupFS.Rollback()
	require.NoError(err)

	// assert both base symlinks point to their respective previous paths
	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir, "/test01.txt"))
	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir)

	symlinkMustExistWithTragetPath(t, base, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir, "/test01.txt"))
	symlinkMustExistWithTragetPath(t, base, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir)

	// never existed before, was created and then rolled back
	mustNotLExist(t, backupFS, "/to_be_removed_symlink")

	mustNotLExist(t, backup, filepath.Join(fileDirRoot, "/file_symlink"))
	mustNotLExist(t, backup, filepath.Join(fileDirRoot, "/directory_symlink"))

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")

}

func TestBackupFS_Mkdir(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/001/002"
	)

	err := mkdir(t, base, fileDirRoot, 0755)
	require.NoError(err)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	err = mkdir(t, backupFS, fileDir2, 0755)
	require.Error(err, "cannot create child directory without having created its parent")

	err = mkdir(t, backupFS, fileDir, 0755)
	require.NoError(err)

	err = mkdir(t, backupFS, fileDir2, 0755)
	require.NoError(err)

	removeAll(t, backupFS, fileDirRoot)

	// /test existed in the base filesystem and has been removed at the end -> upon removal we backup this directory.
	mustLExist(t, backup, fileDirRoot)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_MkdirAll(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/Program Data"
		fileDir     = "/Program Data/001"
		fileDir2    = "/Program Data/001/002"
	)

	// already existing files before we touched the filesystem
	err := mkdir(t, base, fileDirRoot, 0755)
	require.NoError(err)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// at this point writing operations must happen on backupFS
	// and read operations should happen on any of the tree, base, backup or backupFS
	mkdirAll(t, backupFS, fileDir2, 0755)
	removeAll(t, backupFS, fileDir)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Chmod(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		filePath    = filepath.Join(fileDirRoot, "/test_file_chmod.txt")
	)
	createFile(t, base, filePath, "chmod test file")

	// get initial permission bits
	initialFi, err := base.Lstat(filePath)
	require.NoError(err)
	initialMode := initialFi.Mode()

	// snapshot filesystem states
	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// change mod
	expectedNewPerm := fs.FileMode(0644)
	chmod(t, backupFS, filePath, expectedNewPerm)

	// get backed up file permissions
	fi, err := backup.Lstat(filePath)
	require.NoError(err)

	// compare backed up permissions to initial permissions
	backedUpPerm := fi.Mode()
	modeMustBeEqual(t, initialMode, backedUpPerm)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestTime(t *testing.T) {
	require := require.New(t)

	t1 := time.Now()
	nanoBefore := t1.UnixNano()

	t2 := time.Unix(nanoBefore/1000000000, nanoBefore%1000000000)
	require.Equal(t1.UnixNano(), t2.UnixNano())
}

// this helper function is needed in order to test on the local filesystem
// and not in memory
func NewTempDirPrefixFS(rootDir string) *PrefixFS {
	var osFS = NewOSFS()

	tempDir, err := TempDir(osFS, rootDir, fmt.Sprintf("%s-", time.Now().Format("2006-01-02_15-04-05.000")))
	if err != nil {
		log.Fatalln(err)
	}

	var volumeFS FS = osFS

	volume := filepath.VolumeName(tempDir)
	if volume != "" {
		// on linux we do not need this branch
		volumeFS = NewVolumeFS(volume, osFS)
		tempDir = TrimVolume(tempDir)
	}

	pfs, err := NewPrefixFS(volumeFS, tempDir)
	if err != nil {
		panic(err)
	}
	return pfs
}

func NewTestBackupFS() (root, base, backup FS, backupFS *BackupFS) {
	rootPath := CallerPathTmp()
	root = NewTempDirPrefixFS(rootPath)

	err := root.MkdirAll("/base", 0700)
	if err != nil {
		panic(err)
	}

	err = root.MkdirAll("/base/backup", 0700)
	if err != nil {
		panic(err)
	}

	backupFile := "/base/backup.file"
	f, err := root.Create(backupFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = io.WriteString(f, "backup file")
	if err != nil {
		panic(err)
	}

	base, err = NewPrefixFS(root, "/base")
	if err != nil {
		panic(err) // should not happen, becaus epath is not prefixed with a volume
	}
	backup, err = NewPrefixFS(base, "/backup")
	if err != nil {
		panic(err) // should not happen, becaus epath is not prefixed with a volume
	}

	// hide backup locations in base filesystem
	base, err = NewHiddenFS(base, "/backup", "/backup.file")
	if err != nil {
		panic(err)
	}
	backupFS = NewBackupFS(
		base,
		backup,
	)

	return root, base, backup, backupFS
}

func TestCreateFileInSymlinkDir(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkSubDir       = path.Join(symlinkDir, "/systemd/system")
		symlinkFilePath     = path.Join(symlinkSubDir, "test.txt")

		updatedFileContent = "updated_content"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir)
	createFile(t, base, originalFilePath, originalFileContent)

	baseFsState := createFSState(t, base, "/")
	backupFsState := createFSState(t, backup, "/")

	// try creating the directory tree ober a symlinked folder
	createFile(t, backupFS, symlinkFilePath, updatedFileContent)

	err := backupFS.Rollback()
	require.NoError(t, err)

	mustEqualFSState(t, baseFsState, base, "/")
	mustEqualFSState(t, backupFsState, backup, "/")
}

func TestMkdirInSymlinkDir(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir)
	createFile(t, base, originalFilePath, originalFileContent)

	baseFsState := createFSState(t, base, "/")
	backupFsState := createFSState(t, backup, "/")

	// try creating the directory tree ober a symlinked folder
	mkdir(t, backupFS, filepath.Join(symlinkDir, "test_dir"), 0755)

	err := backupFS.Rollback()
	require.NoError(t, err)

	mustEqualFSState(t, baseFsState, base, "/")
	mustEqualFSState(t, backupFsState, backup, "/")
}

func TestRemoveDirInSymlinkDir(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS()

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkSubDir       = "/lib/systemd"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir)
	createFile(t, base, originalFilePath, originalFileContent)

	baseFsState := createFSState(t, base, "/")
	backupFsState := createFSState(t, backup, "/")

	// try creating the directory tree ober a symlinked folder
	removeAll(t, backupFS, symlinkSubDir)

	err := backupFS.Rollback()
	require.NoError(t, err)

	mustEqualFSState(t, baseFsState, base, "/")
	mustEqualFSState(t, backupFsState, backup, "/")
}

func CallerPathTmp(up ...int) string {
	caller := 1
	if len(up) > 0 {
		caller += up[0]
	}
	funcName := strings.TrimPrefix(path.Ext(testutils.CallerFuncName(caller)), ".")
	return testutils.FilePath(filepath.Join("tmp", funcName))
}
