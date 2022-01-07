package backupfs_test

import (
	"errors"
	"os"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/jxsl13/backupfs"
	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func NewTestBackupFs(mockedBase Fs) (backupLayer, backupFs Fs) {
	m := afero.NewMemMapFs()
	backup := backupfs.NewPrefixFs("/backup", m)
	return backup, backupfs.NewBackupFs(mockedBase, backup)
}
func TestMockFsStat(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var (
		filePath    = "/test/mock/01/test.txt"
		expectedErr = errors.New("unknown error")
	)

	// mocked base filesystem
	mf := NewMockFs(mockCtrl)
	mf.EXPECT().Stat(filePath).AnyTimes().Return(nil, expectedErr)

	// backupfs contains a broken basefile system
	backup, fs := NewTestBackupFs(mf)

	_, err := fs.Create(filePath)
	assert.Error(err)
	assert.Equal(err, expectedErr)

	internal.MustNotExist(t, backup, filePath)

	err = fs.Remove(filePath)
	assert.Error(err)
	assert.Equal(err, expectedErr)

	_, err = fs.OpenFile(filePath, os.O_RDWR, 0777)
	assert.Error(err)
	assert.Equal(err, expectedErr)
}

func TestMockFsMkdir(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dirs := []struct {
		Path  string
		Info  os.FileInfo
		Error error
	}{
		{
			"/test",
			internal.CreateMemDir("/test", 0755),
			nil,
		},
		{
			"/test/01",
			internal.CreateMemDir("/test/01", 0755),
			nil,
		},
		{
			"/test/01/mock",
			internal.CreateMemDir("/test/01/mock", 0755),
			os.ErrNotExist,
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
		assert.Equal(err, d.Error)
		if err != nil {
			internal.MustNotExist(t, backup, d.Path)
		} else {
			internal.MustExist(t, backup, d.Path)
		}
	}

}
