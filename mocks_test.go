package backupfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -package=backupfs -source=filesystem.go -destination=./mock_test.go FS

func NewTestMockBackupFS(mockedBase FS) (backupLayer, BackupFS FS) {
	rootPath := CallerPathTmp()
	root := NewTempDirPrefixFS(rootPath)

	backupDir := "/backup"
	err := root.MkdirAll(backupDir, 0700)
	if err != nil {
		panic(err)
	}
	backup := NewPrefixFS(root, backupDir)
	return backup, NewBackupFS(mockedBase, backup)
}
func TestMockFSLstat(t *testing.T) {
	require := require.New(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var (
		filePath    = filepath.Clean("/test/mock/01/test.txt")
		expectedErr = errors.New("unknown error")
	)

	// mocked base filesystem
	mf := NewMockFS(mockCtrl)

	// lstat is called for file paths
	mf.EXPECT().Lstat(filePath).AnyTimes().Return(nil, expectedErr)

	// also expect calls to all sub directories
	// in order to track their state AT LEAST ONCE.
	IterateDirTree(filepath.Dir(filePath), func(subdirPath string) (bool, error) {
		mf.EXPECT().Lstat(subdirPath).AnyTimes().Return(nil, expectedErr)
		return true, nil
	})

	// BackupFS contains a broken basefile system
	backup, fsys := NewTestMockBackupFS(mf)

	_, err := fsys.Create(filePath)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))

	mustNotExist(t, backup, filePath)

	err = fsys.Remove(filePath)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))

	_, err = fsys.OpenFile(filePath, os.O_RDWR, 0777)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))
}

func createFileInfo(path string, perm fs.FileMode) fs.FileInfo {
	return &fileInfo{
		name:    filepath.Clean(path),
		mode:    perm,
		modTime: time.Now(),
	}
}

func createDirInfo(path string, perm fs.FileMode) fs.FileInfo {
	return createFileInfo(path, perm|fs.ModeDir)
}

type fileInfo struct {
	name    string
	mode    fs.FileMode
	modTime time.Time
}

func (fi *fileInfo) Name() string {
	return fi.name
}
func (fi *fileInfo) Size() int64 {
	return 0
}
func (fi *fileInfo) Mode() fs.FileMode {
	return fi.mode
}
func (fi *fileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi *fileInfo) IsDir() bool {
	return fi.mode.IsDir()
}
func (fi *fileInfo) Sys() any {
	return nil
}

func TestMockFSMkdir(t *testing.T) {
	require := require.New(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dirs := []struct {
		Path  string
		Info  fs.FileInfo
		Error error
	}{
		{
			filepath.Clean("/test"),
			createDirInfo("/test", 0755),
			nil,
		},
		{
			filepath.Clean("/test/01"),
			createDirInfo("/test/01", 0755),
			nil,
		},
		{
			filepath.Clean("/test/01/mock"),
			createDirInfo("/test/01/mock", 0755),
			syscall.ENOENT,
		},
	}

	// mocked base filesystem
	mf := NewMockFS(mockCtrl)
	for _, d := range dirs {
		mf.EXPECT().Lstat(d.Path).AnyTimes().Return(d.Info, d.Error)
		if d.Error != nil {
			mf.EXPECT().MkdirAll(d.Path, gomock.Any()).Return(d.Error)
		} else {
			mf.EXPECT().MkdirAll(d.Path, gomock.Any()).Return(nil)

		}
	}

	// BackupFS contains a broken basefile system
	backup, fs := NewTestMockBackupFS(mf)

	for _, d := range dirs {
		expectedErr := d.Error
		err := fs.MkdirAll(d.Path, 0755)
		require.ErrorIs(err, expectedErr, "expected error %v, got error %v", expectedErr, err)
		if err != nil {
			mustNotExist(t, backup, d.Path)
		} else {
			mustExist(t, backup, d.Path)
		}
	}

}
