package backupfs

import (
	"io/fs"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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

		filePath = "/usr/test.txt"
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
	_, err := resolvePath(base, filePath)
	require.NoError(t, err)
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
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkSubDir       = path.Join(symlinkDir, "/systemd/system")
		symlinkFilePath     = path.Join(symlinkSubDir, "test.txt")
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir) // create absolute symlink
	createFile(t, base, originalFilePath, originalFileContent)

	resolvedPath, err := resolvePath(base, symlinkFilePath)
	require.NoError(t, err)
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

	resolvedPath, err := resolvePath(base, symlinkFilePath)
	require.NoError(t, err)
	require.Equal(t, originalFilePath, resolvedPath)
}

func TestResolvePathWithCacheWithAbsoluteSymlinkTwice(t *testing.T) {
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
	createSymlink(t, base, originalLinkedDir, symlinkDir) // create absolute symlink
	createFile(t, base, originalFilePath, originalFileContent)

	var (
		fiCache   = make(map[string]fs.FileInfo)
		pathcache = make(map[string]string)
	)

	resolvedPath, err := resolvePathWithCache(base, symlinkFilePath, fiCache, pathcache)
	require.NoError(t, err)
	require.Equal(t, originalFilePath, resolvedPath)

	resolvedPath, err = resolvePathWithCache(base, symlinkFilePath, fiCache, pathcache)
	require.NoError(t, err)
	require.Equal(t, originalFilePath, resolvedPath)

}

func TestResolvePathWithCacheWithRelativeSymlinkTwice(t *testing.T) {
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

	var (
		fiCache   = make(map[string]fs.FileInfo)
		pathcache = make(map[string]string)
	)

	resolvedPath, err := resolvePathWithCache(base, symlinkFilePath, fiCache, pathcache)
	require.NoError(t, err)
	require.Equal(t, originalFilePath, resolvedPath)

	resolvedPath, err = resolvePathWithCache(base, symlinkFilePath, fiCache, pathcache)
	require.NoError(t, err)
	require.Equal(t, originalFilePath, resolvedPath)
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
