package backupfs

import (
	"testing"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestPrefixFS_FileRootDirectoryName(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)
	osfs := NewOSFS()

	absRootDir := testutils.AbsFilePath(t, separator)

	osf, err := osfs.Open(absRootDir)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, osf.Close())
	}()

	osName := osf.Name()

	f, err := base.Open(absRootDir)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, f.Close())
	}()
	prefixName := f.Name()

	assert.Equal(t, osName, prefixName)
}

func TestPrefixFS_FileRootDirectoryStatName(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)
	osfs := NewOSFS()

	absRootDir := testutils.AbsFilePath(t, separator)

	osf, err := osfs.Open(absRootDir)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, osf.Close())
	}()

	osStat, err := osf.Stat()
	if !assert.NoError(t, err) {
		return
	}
	osName := osStat.Name()

	f, err := base.Open(absRootDir)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, f.Close())
	}()

	prefixStat, err := f.Stat()
	if !assert.NoError(t, err) {
		return
	}
	prefixName := prefixStat.Name()

	assert.Equal(t, osName, prefixName)
}
