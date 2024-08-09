package backupfs

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePathWithFileThatDoesntExist(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	var (
		originalSubDir   = "/usr/lib/systemd/system"
		originalFilePath = "/usr/lib/systemd/system/test.txt" // file is never created
		symlinkDir       = "/lib"
		symlinkFilePath  = "/lib/systemd/system/test.txt"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, "../usr/lib", symlinkDir) // create relative symlink

	// resolve file that does not exist
	resolvedPath, found, err := resolvePathWithFound(base, symlinkFilePath)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, originalFilePath, resolvedPath)
}

func TestResolveCircularSymlinkPath(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	var (
		folders = "/usr/lib/systemd"

		symlink1  = "/lib"
		pointsAt1 = "/usr/lib"

		symlink2  = "/usr/lib/systemd/system"
		pointsAt2 = "/usr"

		filePath        = "/usr/test.txt"
		symlinkFilePath = "/lib/systemd/system/test.txt"
	)

	// prepare existing files
	mkdirAll(t, base, folders, 0755)
	createFile(t, base, filePath, "test_content")

	// create circular symlink
	// /lib -> /usr/lib
	// /usr/lib/systemd/system -> /usr
	createSymlink(t, base, pointsAt1, symlink1) // create absolute symlink
	createSymlink(t, base, pointsAt2, symlink2)

	// there is no real problem of resolving circular symlinks, because the provided path is
	// limited and has no recursion in itself
	resolvedPath, found, err := resolvePathWithFound(base, symlinkFilePath)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, filePath, resolvedPath)
}

func TestResolvePathWithAbsoluteSymlink(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = "/usr/lib/systemd/system"
		originalFilePath    = "/usr/lib/systemd/system/test.txt"
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkFilePath     = "/lib/systemd/system/test.txt"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir) // create absolute symlink
	createFile(t, base, originalFilePath, originalFileContent)

	resolvedPath, found, err := resolvePathWithFound(base, symlinkFilePath)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, originalFilePath, resolvedPath)
}

func TestResolvePathWithRelativeSymlink(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkSubDir       = path.Join(symlinkDir, "/systemd/system")
		symlinkFilePath     = path.Join(symlinkSubDir, "test.txt")
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, "../usr/lib", symlinkDir) // create relative symlink
	createFile(t, base, originalFilePath, originalFileContent)

	resolvedPath, found, err := resolvePathWithFound(base, symlinkFilePath)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, originalFilePath, resolvedPath)
}

func TestResolveFilePathWithRelativeSymlink(t *testing.T) {
	t.Parallel()

	var (
		basePrefix   = "/base"
		backupPrefix = "/backup"
	)

	_, base, _, _ := NewTestBackupFS(basePrefix, backupPrefix)

	var (
		originalSubDir      = "/usr/lib/systemd/system"
		originalFilePath    = "/usr/lib/systemd/system/test.txt"
		originalFileContent = "test_content"
		symlinkFile         = "/usr/lib/linked_file"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createFile(t, base, originalFilePath, originalFileContent)
	createSymlink(t, base, "../../usr/lib/systemd/system/test.txt", symlinkFile) // create relative symlink

	resolvedPath, found, err := resolvePathWithFound(base, symlinkFile)
	require.NoError(t, err)
	require.True(t, found)

	// the final file is a symlink that points to a different file.
	// we only want to resolve the path leading to the symlink, not the symlink itself.
	require.Equal(t, symlinkFile, resolvedPath)
}

func TestIterateDirTreeAbsolute(t *testing.T) {
	filePath := filepath.Join(separator, "a", "b", "c", "d", "test.txt")

	parts := make([]string, 0, 6)
	_, err := IterateDirTree(filePath, func(s string) (proceed bool, err error) {
		parts = append(parts, s)
		return true, nil
	})
	require.NoError(t, err)

	expected := []string{
		separator,
		filepath.Join(separator, "a"),
		filepath.Join(separator, "a", "b"),
		filepath.Join(separator, "a", "b", "c"),
		filepath.Join(separator, "a", "b", "c", "d"),
		filepath.Join(separator, "a", "b", "c", "d", "test.txt"),
	}
	require.Equal(t, expected, parts)
}

func TestIterateDirTreeRelative(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join("a", "b", "c", "d", "test.txt")

	parts := make([]string, 0, 6)
	_, err := IterateDirTree(filePath, func(s string) (proceed bool, err error) {
		parts = append(parts, s)
		return true, nil
	})
	require.NoError(t, err)

	expected := []string{
		filepath.Join("a"),
		filepath.Join("a", "b"),
		filepath.Join("a", "b", "c"),
		filepath.Join("a", "b", "c", "d"),
		filepath.Join("a", "b", "c", "d", "test.txt"),
	}
	require.Equal(t, expected, parts)
}

func TestIterateDirTreeEmpty(t *testing.T) {
	t.Parallel()

	parts := make([]string, 0, 1)
	_, err := IterateDirTree("", func(s string) (proceed bool, err error) {
		parts = append(parts, s)
		return true, nil
	})
	require.NoError(t, err)

	expected := []string{}
	require.Equal(t, expected, parts)
}
