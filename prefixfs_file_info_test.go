package backupfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixFSFileInfoRootName(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS()

	rootDir := separator

	fi, err := base.Stat(rootDir)
	require.NoError(t, err)
	require.Equal(t, rootDir, fi.Name())

	fi, err = base.Lstat(rootDir)
	require.NoError(t, err)
	require.Equal(t, rootDir, fi.Name())

}
