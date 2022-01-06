package backupfs

import (
	"io/ioutil"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var (
	mm afero.Fs
	mo sync.Once
)

func NewTestMemMapFs() afero.Fs {
	if mm != nil {
		return mm
	}

	mo.Do(func() {
		mm = afero.NewMemMapFs()
	})

	return mm
}

func NewTestPrefixFs(prefix string) *PrefixFs {
	return NewPrefixFs(prefix, NewTestMemMapFs())
}

func TestBackupFsCreate(t *testing.T) {
	assert := assert.New(t)
	root := NewTestPrefixFs("/")
	base := NewTestPrefixFs("/base")
	backup := NewTestPrefixFs("/backup")
	fs := NewBackupFs(base, backup)

	err := base.MkdirAll("/test/01/", 0755)
	assert.NoError(err)

	initialText := "test_01"

	f, err := base.Create("/test/01/test_01.txt")
	assert.NoError(err)

	_, err = f.WriteString(initialText)
	assert.NoError(err)
	err = f.Close()
	assert.NoError(err)

	f, err = fs.Create("/test/01/test_01.txt")
	assert.NoError(err)

	_, err = f.WriteString("test_01_owerwritten")
	assert.NoError(err)

	f, err = root.Open("backup/test/01/test_01.txt")
	assert.NoError(err)

	b, err := ioutil.ReadAll(f)
	assert.NoError(err)

	assert.Equal(string(b), initialText)

}
