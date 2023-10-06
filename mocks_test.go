package backupfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/jxsl13/backupfs/fsi"
	"github.com/jxsl13/backupfs/fsutils"
	"github.com/jxsl13/backupfs/internal/mem"
	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func NewTestMockBackupFs(mockedBase fsi.Fs) (backupLayer, backupFs fsi.Fs) {
	m := mem.NewMemMapFs()
	backup := NewPrefixFs("/backup", m)
	return backup, NewBackupFs(mockedBase, backup)
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
	fsutils.IterateDirTree(filepath.Dir(filePath), func(subdirPath string) error {
		mf.EXPECT().Stat(subdirPath).AnyTimes().Return(nil, expectedErr)
		return nil
	})

	// backupfs contains a broken basefile system
	backup, fs := NewTestMockBackupFs(mf)

	_, err := fs.Create(filePath)
	require.Error(err)
	require.True(errors.Is(err, expectedErr))

	testutils.MustNotExist(t, backup, filePath)

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
		Info  fs.FileInfo
		Error error
	}{
		{
			filepath.Clean("/test"),
			testutils.CreateMemDir("/test", 0755),
			nil,
		},
		{
			filepath.Clean("/test/01"),
			testutils.CreateMemDir("/test/01", 0755),
			nil,
		},
		{
			filepath.Clean("/test/01/mock"),
			testutils.CreateMemDir("/test/01/mock", 0755),
			syscall.ENOENT,
		},
	}

	// mocked base filesystem
	mf := NewMockFs(mockCtrl)
	//proxyFs := mem.NewMemMapFs()
	for _, d := range dirs {
		mf.EXPECT().Stat(d.Path).AnyTimes().Return(d.Info, d.Error)
		if d.Error == nil {
			mf.EXPECT().MkdirAll(d.Path, gomock.Any()).Return(nil)
		} else {
			mf.EXPECT().MkdirAll(d.Path, gomock.Any()).Return(d.Error)
		}

	}

	// backupfs contains a broken basefile system
	backup, fs := NewTestMockBackupFs(mf)

	for _, d := range dirs {
		err := fs.MkdirAll(d.Path, 0755)
		require.Truef(errors.Is(err, d.Error), "expected error %v, got error %v", d.Error, err)
		if err != nil {
			testutils.MustNotExist(t, backup, d.Path)
		} else {
			testutils.MustExist(t, backup, d.Path)
		}
	}

}
