package backupfs

import (
	"testing"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestOSFS_RemoveFileSymlink(t *testing.T) {
	t.Parallel()

	osfs := NewOSFS()
	rootPath, err := TempDir(osfs, testutils.FuncName())
	require.NoError(t, err)

	mkdirAll(t, osfs, rootPath, 0o700)

	const (
		dir     = "/dir"
		file    = "/dir/file.txt"
		symlink = "/link_to_file"
	)

	mkdir(t, osfs, dir, 0o700)
	createFile(t, osfs, file, "content")
	createSymlink(t, osfs, file, symlink)

	err = osfs.Remove(symlink)
	require.NoError(t, err)

	mustNotLExist(t, osfs, symlink)
	mustLExist(t, osfs, file)
}

func TestOSFS_RemoveASllFileSymlink(t *testing.T) {
	t.Parallel()

	osfs := NewOSFS()
	rootPath, err := TempDir(osfs, testutils.FuncName())
	require.NoError(t, err)

	mkdirAll(t, osfs, rootPath, 0o700)

	const (
		dir     = "/dir"
		file    = "/dir/file.txt"
		symlink = "/link_to_file"
	)

	mkdir(t, osfs, dir, 0o700)
	createFile(t, osfs, file, "content")
	createSymlink(t, osfs, file, symlink)

	err = osfs.RemoveAll(symlink)
	require.NoError(t, err)

	mustNotLExist(t, osfs, symlink)
	mustLExist(t, osfs, file)
}
