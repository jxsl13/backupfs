package backupfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefixFSFileRootDirectoryName(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	rootDir := separator

	f, err := base.Open(rootDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	assert.Equal(t, rootDir, f.Name())
}
