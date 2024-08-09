package backupfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixFSFileInfoRootName(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)
	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	rootDir := separator

	fi, err := base.Stat(rootDir)
	require.NoError(t, err)
	require.Equal(t, rootDir, fi.Name())

	fi, err = base.Lstat(rootDir)
	require.NoError(t, err)
	require.Equal(t, rootDir, fi.Name())

}
