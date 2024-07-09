package backupfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHiddenFS_CountFiles(t *testing.T) {
	t.Parallel()

	hiddenDirParent, hiddenDir, _, base, fsys := SetupTempDirHiddenFSTest(t)

	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 1)
}

func TestHiddenFS_Create(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fsys := SetupTempDirHiddenFSTest(t)

	// try doing stuff in the hidden directory
	_, err := fsys.Create(hiddenDir)
	require.Error(err)

	_, err = fsys.Create(filepath.Join(hiddenDir, "test.txt"))
	require.ErrorIs(err, os.ErrPermission)

	_, err = fsys.Create(filepath.Join(hiddenDir, hiddenFile))
	require.ErrorIs(err, os.ErrPermission)

	createFile(t, fsys, filepath.Join(hiddenDirParent, "test.txt"), "file content")

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 2)
}

func TestHiddenFS_Mkdir(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fsys := SetupTempDirHiddenFSTest(t)

	// try doing stuff in the hidden directory
	err := fsys.Mkdir(hiddenDir, 0775)
	require.ErrorIs(err, os.ErrPermission)

	err = fsys.Mkdir(filepath.Join(hiddenDir, hiddenFile), 0775)
	require.ErrorIs(err, os.ErrPermission)

	err = fsys.Mkdir(filepath.Join(hiddenDir, hiddenFile, "should_not_exist"), 0775)
	require.ErrorIs(err, os.ErrPermission)

	mkdir(t, fsys, filepath.Join(hiddenDirParent, "should_exist"), 0775)

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)

	// created another directory next to hidenDir
	countFiles(t, fsys, hiddenDirParent, 2)
}

func TestHiddenFS_MkdirAll(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fsys := SetupTempDirHiddenFSTest(t)

	// try doing stuff in the hidden directory
	mkdirAll(t, fsys, filepath.Join(hiddenDirParent, "does_not_exist_yet"), 0775)

	err := fsys.MkdirAll(hiddenDir, 0775)
	require.ErrorIs(err, os.ErrPermission)

	err = fsys.MkdirAll(filepath.Join(hiddenDir, hiddenFile), 0775)
	require.ErrorIs(err, os.ErrPermission)

	mkdirAll(t, fsys, filepath.Join(hiddenDir+"_random_suffix", "should_be_created"), 0775)
	mkdirAll(t, fsys, filepath.Join(hiddenDir[:len(hiddenDir)-2], "should_be_created"), 0775)

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 6)
}

func TestHiddenFS_OpenFile(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fsys := SetupTempDirHiddenFSTest(t)

	// try doing stuff in the hidden directory
	createFile(t, fsys, filepath.Join(hiddenDir[:len(hiddenDir)-2], "should_be_created"), "text")
	createFile(t, fsys, filepath.Join(hiddenDir+"_random_suffix", "should_be_created"), "text")
	openFile(t, fsys, filepath.Join(hiddenDirParent, "does_not_exist_yet"), "test", 0775)

	_, err := fsys.OpenFile(hiddenDir, os.O_RDONLY, 0755)
	require.ErrorIs(err, os.ErrNotExist)

	_, err = fsys.OpenFile(filepath.Join(hiddenDir, hiddenFile), os.O_RDONLY, 0755)
	require.ErrorIs(err, os.ErrNotExist)

	_, err = fsys.Create(filepath.Join(hiddenDir, hiddenFile))
	require.ErrorIs(err, os.ErrPermission)

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 6)
}

func TestHiddenFS_Remove(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fsys := SetupTempDirHiddenFSTest(t)

	// try doing stuff in the hidden directory
	err := fsys.Remove(hiddenDir)
	require.ErrorIs(err, os.ErrNotExist)

	err = fsys.Remove(filepath.Join(hiddenDir, hiddenFile))
	require.ErrorIs(err, os.ErrNotExist)

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 1)
}

func TestHiddenFS_RemoveAll(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, _, base, fsys := SetupTempDirHiddenFSTest(t)

	createFile(t, fsys, filepath.Join(hiddenDir[:len(hiddenDir)-2], "should_be_created"), "text")
	createFile(t, fsys, filepath.Join(hiddenDir+"_random_suffix", "should_be_created"), "text")
	openFile(t, fsys, filepath.Join(hiddenDirParent, "does_not_exist_yet"), "test", 0775)

	err := fsys.RemoveAll(hiddenDirParent)
	require.NoError(err)

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 1)
}

func TestHiddenFSSymlink(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fsys := SetupTempDirHiddenFSTest(t)

	var (
		oldpath = filepath.Join(hiddenDirParent, "oldpath")
		newpath = filepath.Join(hiddenDirParent, "newpath")

		hiddenNewpath = filepath.Join(hiddenDir, "newpath")
	)

	createFile(t, fsys, oldpath, "text")

	// cannot create symlink i hidden dir
	err := fsys.Symlink(oldpath, hiddenDir)
	require.ErrorIs(err, os.ErrPermission)

	err = fsys.Symlink(oldpath, hiddenNewpath)
	require.ErrorIs(err, os.ErrPermission)

	// cannot symlink into hidden dir
	err = fsys.Symlink(filepath.Join(hiddenDir, hiddenFile), newpath)
	require.ErrorIs(err, os.ErrPermission)

	// cannot symlink into hidden dir via relative path
	err = fsys.Symlink(filepath.Join("../../var/opt/backups", hiddenFile), newpath)
	require.ErrorIs(err, os.ErrPermission)

	// able to create symlinks outside of hidden dir
	err = fsys.Symlink("../parentdirfile", newpath+"-1")
	require.NoError(err)

	err = fsys.Symlink(hiddenDirParent, newpath+"-2")
	require.NoError(err)

	// at the end the hidden directory should containthe same number of files as before
	countFiles(t, base, hiddenDir, 2)
	countFiles(t, fsys, hiddenDirParent, 4)
}

func NewTestTempDirHiddenFS(hiddenPaths ...string) (base FS, hfs *HiddenFS) {
	return newTestTempDirHiddenFS(0, hiddenPaths...)
}

func newTestTempDirHiddenFS(caller int, hiddenPaths ...string) (base FS, hfs *HiddenFS) {
	rootPath := CallerPathTmp(caller)
	root := NewTempDirPrefixFS(rootPath)

	hidden := "/hidden"
	err := root.MkdirAll(hidden, 0700)
	if err != nil {
		panic(err)
	}
	base = NewPrefixFS(root, hidden)
	return base, NewHiddenFS(base, hiddenPaths...)
}

func SetupTempDirHiddenFSTest(t *testing.T) (hiddenDirParent, hiddenDir, hiddenFile string, base FS, fs *HiddenFS) {
	hiddenDirParent = "/var/opt"
	hiddenDir = "/var/opt/backups"
	hiddenFile = "hidden_file.txt"

	// prepare base filesystem before using the hidden fs layer
	base, fs = newTestTempDirHiddenFS(1, hiddenDir)

	mkdir(t, base, hiddenDirParent, 0775)
	mkdirAll(t, base, hiddenDir, 0775)
	createFile(t, base, filepath.Join(hiddenDir, hiddenFile), "hidden content")
	return
}
