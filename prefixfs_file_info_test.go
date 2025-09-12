package backupfs

import (
	"testing"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestPrefixFS_FileInfoRootName(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)
	osfs := NewOSFS()

	rootDir := separator
	absRootDir := testutils.AbsFilePath(t, rootDir)

	// compare to os package behavior
	osfi, err := osfs.Stat(absRootDir)
	require.NoError(t, err)
	require.Equal(t, rootDir, osfi.Name())

	fi, err := base.Stat(absRootDir)
	require.NoError(t, err)
	require.Equal(t, osfi.Name(), fi.Name())

	osfi, err = osfs.Lstat(absRootDir)
	require.NoError(t, err)

	fi, err = base.Lstat(absRootDir)
	require.NoError(t, err)
	require.Equal(t, osfi.Name(), fi.Name())
}
