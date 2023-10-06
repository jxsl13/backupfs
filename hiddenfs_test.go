package backupfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jxsl13/backupfs/fsi"
	"github.com/jxsl13/backupfs/internal/mem"
	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func NewTestTempdirHiddenFs(hiddenPaths ...string) (base fsi.Fs, hfs *HiddenFs) {
	base = NewTempdirPrefixFs("/hidefs")
	return base, NewHiddenFs(base, hiddenPaths...)
}

// creates a new memmap for every test
// contrary to the other function that only creates a single memmap
func NewTestMemMapHiddenFs(hiddenPaths ...string) (base fsi.Fs, hfs *HiddenFs) {
	base = mem.NewMemMapFs()
	return base, NewHiddenFs(base, hiddenPaths...)
}

func SetupTempDirHiddenFsTest(t *testing.T) (hiddenDirParent, hiddenDir, hiddenFile string, base fsi.Fs, fs *HiddenFs) {
	hiddenDirParent = "/var/opt"
	hiddenDir = "/var/opt/backups"
	hiddenFile = "hidden_file.txt"

	// prepare base filesystem before using the hidden fs layer
	base, fs = NewTestTempdirHiddenFs(hiddenDir)

	testutils.Mkdir(t, base, hiddenDirParent, 0775)
	testutils.MkdirAll(t, base, hiddenDir, 0775)
	testutils.CreateFile(t, base, filepath.Join(hiddenDir, hiddenFile), "hidden content")

	return
}

func SetupMemMapHiddenFsTest(t *testing.T) (hiddenDirParent, hiddenDir, hiddenFile string, base fsi.Fs, fs *HiddenFs) {
	hiddenDirParent = "/var/opt"
	hiddenDir = "/var/opt/backups"
	hiddenFile = "hidden_file.txt"

	// prepare base filesystem before using the hidden fs layer
	base, fs = NewTestMemMapHiddenFs(hiddenDir)

	testutils.Mkdir(t, base, hiddenDirParent, 0775)
	testutils.MkdirAll(t, base, hiddenDir, 0775)
	testutils.CreateFile(t, base, filepath.Join(hiddenDir, hiddenFile), "hidden content")

	return
}

func TestCountFiles(t *testing.T) {

	hiddenDirParent, hiddenDir, _, base, fs := SetupMemMapHiddenFsTest(t)

	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 1)
}

func TestHiddenFsCreate(t *testing.T) {

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fs := SetupMemMapHiddenFsTest(t)

	// try doing stuff in the hidden directory
	_, err := fs.Create(hiddenDir)
	require.Error(err)

	_, err = fs.Create(filepath.Join(hiddenDir, "test.txt"))
	require.ErrorIs(err, os.ErrPermission)

	_, err = fs.Create(filepath.Join(hiddenDir, hiddenFile))
	require.ErrorIs(err, os.ErrPermission)

	testutils.CreateFile(t, fs, filepath.Join(hiddenDirParent, "test.txt"), "file content")

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 2)
}

func TestHiddenFsMkdir(t *testing.T) {

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fs := SetupMemMapHiddenFsTest(t)

	// try doing stuff in the hidden directory
	err := fs.Mkdir(hiddenDir, 0775)
	require.ErrorIs(err, os.ErrPermission)

	err = fs.Mkdir(filepath.Join(hiddenDir, hiddenFile), 0775)
	require.ErrorIs(err, os.ErrPermission)

	err = fs.Mkdir(filepath.Join(hiddenDir, hiddenFile, "should_not_exist"), 0775)
	require.ErrorIs(err, os.ErrPermission)

	testutils.Mkdir(t, fs, filepath.Join(hiddenDirParent, "should_exist"), 0775)

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)

	// created another directory next to hidenDir
	testutils.CountFiles(t, fs, hiddenDirParent, 2)
}

func TestHiddenFsMkdirAll(t *testing.T) {

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fs := SetupMemMapHiddenFsTest(t)

	// try doing stuff in the hidden directory
	testutils.MkdirAll(t, fs, filepath.Join(hiddenDirParent, "does_not_exist_yet"), 0775)

	err := fs.MkdirAll(hiddenDir, 0775)
	require.ErrorIs(err, os.ErrPermission)

	err = fs.MkdirAll(filepath.Join(hiddenDir, hiddenFile), 0775)
	require.ErrorIs(err, os.ErrPermission)

	testutils.MkdirAll(t, fs, filepath.Join(hiddenDir+"_random_suffix", "should_be_created"), 0775)
	testutils.MkdirAll(t, fs, filepath.Join(hiddenDir[:len(hiddenDir)-2], "should_be_created"), 0775)

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 6)
}

func TestHiddenFsOpenFile(t *testing.T) {

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fs := SetupMemMapHiddenFsTest(t)

	// try doing stuff in the hidden directory
	testutils.CreateFile(t, fs, filepath.Join(hiddenDir[:len(hiddenDir)-2], "should_be_created"), "text")
	testutils.CreateFile(t, fs, filepath.Join(hiddenDir+"_random_suffix", "should_be_created"), "text")
	testutils.OpenFile(t, fs, filepath.Join(hiddenDirParent, "does_not_exist_yet"), "test", 0775)

	_, err := fs.OpenFile(hiddenDir, os.O_RDONLY, 0755)
	require.ErrorIs(err, os.ErrNotExist)

	_, err = fs.OpenFile(filepath.Join(hiddenDir, hiddenFile), os.O_RDONLY, 0755)
	require.ErrorIs(err, os.ErrNotExist)

	_, err = fs.Create(filepath.Join(hiddenDir, hiddenFile))
	require.ErrorIs(err, os.ErrPermission)

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 6)
}

func TestHiddenFsRemove(t *testing.T) {

	require := require.New(t)
	hiddenDirParent, hiddenDir, hiddenFile, base, fs := SetupMemMapHiddenFsTest(t)

	// try doing stuff in the hidden directory
	err := fs.Remove(hiddenDir)
	require.ErrorIs(err, os.ErrNotExist)

	err = fs.Remove(filepath.Join(hiddenDir, hiddenFile))
	require.ErrorIs(err, os.ErrNotExist)

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 1)
}

func TestHiddenFsRemoveAll(t *testing.T) {
	require := require.New(t)

	hiddenDirParent, hiddenDir, _, base, fs := SetupMemMapHiddenFsTest(t)

	testutils.CreateFile(t, fs, filepath.Join(hiddenDir[:len(hiddenDir)-2], "should_be_created"), "text")
	testutils.CreateFile(t, fs, filepath.Join(hiddenDir+"_random_suffix", "should_be_created"), "text")
	testutils.OpenFile(t, fs, filepath.Join(hiddenDirParent, "does_not_exist_yet"), "test", 0775)

	err := fs.RemoveAll(hiddenDirParent)
	require.NoError(err)

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 1)
}

func TestHiddenFsSymlink(t *testing.T) {
	require := require.New(t)

	hiddenDirParent, hiddenDir, hiddenFile, base, fs := SetupTempDirHiddenFsTest(t)

	var (
		oldpath = filepath.Join(hiddenDirParent, "oldpath")
		newpath = filepath.Join(hiddenDirParent, "newpath")

		hiddenNewpath = filepath.Join(hiddenDir, "newpath")
	)

	testutils.CreateFile(t, fs, oldpath, "text")

	// cannot create symlink i hidden dir
	err := fs.Symlink(oldpath, hiddenDir)
	require.ErrorIs(err, os.ErrPermission)

	err = fs.Symlink(oldpath, hiddenNewpath)
	require.ErrorIs(err, os.ErrPermission)

	// cannot symlink into hidden dir
	err = fs.Symlink(filepath.Join(hiddenDir, hiddenFile), newpath)
	require.ErrorIs(err, os.ErrPermission)

	// cannot symlink into hidden dir via relative path
	err = fs.Symlink(filepath.Join("../../var/opt/backups", hiddenFile), newpath)
	require.ErrorIs(err, os.ErrPermission)

	// able to create symlinks outside of hidden dir
	err = fs.Symlink("../parentdirfile", newpath+"-1")
	require.NoError(err)

	err = fs.Symlink(hiddenDirParent, newpath+"-2")
	require.NoError(err)

	// at the end the hidden directory should containthe same number of files as before
	testutils.CountFiles(t, base, hiddenDir, 2)
	testutils.CountFiles(t, fs, hiddenDirParent, 4)
}
