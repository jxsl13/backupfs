package backupfs_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/jxsl13/backupfs"
	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func NewTestBackupFs(mockedBase Fs) (backupLayer, backupFs Fs) {
	m := afero.NewMemMapFs()
	backup := backupfs.NewPrefixFs("/backup", m)
	return backup, backupfs.NewBackupFs(mockedBase, backup)
}
func TestMockFsStat(t *testing.T) {
	require := require.New(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var (
		filePath    = filepath.Clean("/test/mock/01/test.txt")
		expectedErr = errors.New("unknown error")
	)

	// mocked base filesystem
	mf := NewMockFs(mockCtrl)
	mf.EXPECT().Stat(filePath).AnyTimes().Return(nil, expectedErr)

	// also expect calls to all sub directories
	// in order to track their state AT LEAST ONCE.
	internal.IterateDirTree(filepath.Dir(filePath), func(subdirPath string) error {
		mf.EXPECT().Stat(subdirPath).AnyTimes().Return(nil, expectedErr)
		return nil
	})

	// backupfs contains a broken basefile system
	backup, fs := NewTestBackupFs(mf)

	_, err := fs.Create(filePath)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))

	internal.MustNotExist(t, backup, filePath)

	err = fs.Remove(filePath)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))

	_, err = fs.OpenFile(filePath, os.O_RDWR, 0777)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))
}

func TestMockFsMkdir(t *testing.T) {
	require := require.New(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dirs := []struct {
		Path  string
		Info  os.FileInfo
		Error error
	}{
		{
			filepath.Clean("/test"),
			internal.CreateMemDir("/test", 0755),
			nil,
		},
		{
			filepath.Clean("/test/01"),
			internal.CreateMemDir("/test/01", 0755),
			nil,
		},
		{
			filepath.Clean("/test/01/mock"),
			internal.CreateMemDir("/test/01/mock", 0755),
			syscall.ENOENT,
		},
	}

	// mocked base filesystem
	mf := NewMockFs(mockCtrl)
	//proxyFs := afero.NewMemMapFs()
	for _, d := range dirs {
		mf.EXPECT().Stat(d.Path).AnyTimes().Return(d.Info, d.Error)
		if d.Error == nil {
			mf.EXPECT().MkdirAll(d.Path, gomock.Any()).Return(nil)
		} else {
			mf.EXPECT().MkdirAll(d.Path, gomock.Any()).Return(d.Error)
		}

	}

	// backupfs contains a broken basefile system
	backup, fs := NewTestBackupFs(mf)

	for _, d := range dirs {
		err := fs.MkdirAll(d.Path, 0755)
		require.Equal(err, d.Error)
		if err != nil {
			internal.MustNotExist(t, backup, d.Path)
		} else {
			internal.MustExist(t, backup, d.Path)
		}
	}

}
