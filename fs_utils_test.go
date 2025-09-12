package backupfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestUtils_ResolvePathWithFileThatDoesntExist(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)

	var (
		err              error
		originalSubDir   = testutils.AbsFilePath(t, "/usr/lib/systemd/system")
		originalFilePath = testutils.AbsFilePath(t, "/usr/lib/systemd/system/test.txt") // file is never created
		symlinkDir       = testutils.AbsFilePath(t, "/lib")
		symlinkFilePath  = testutils.AbsFilePath(t, "/lib/systemd/system/test.txt")
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, "../usr/lib", symlinkDir) // create relative symlink

	// resolve file that does not exist
	resolvedPath, found, err := resolvePathWithFound(base, symlinkFilePath)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, filepath.FromSlash(originalFilePath), resolvedPath)
}

func TestUtils_ResolveCircularSymlinkPath(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)

	var (
		folders = testutils.AbsFilePath(t, "/usr/lib/systemd")

		symlink1  = testutils.AbsFilePath(t, "/lib")
		pointsAt1 = testutils.AbsFilePath(t, "/usr/lib")

		symlink2  = testutils.AbsFilePath(t, "/usr/lib/systemd/system")
		pointsAt2 = testutils.AbsFilePath(t, "/usr")

		filePath        = testutils.AbsFilePath(t, "/usr/test.txt")
		symlinkFilePath = testutils.AbsFilePath(t, "/lib/systemd/system/test.txt")
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
	require.Equal(t, filepath.FromSlash(filePath), resolvedPath)
}

func TestUtils_ResolvePathWithAbsoluteSymlink(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)

	var (
		originalLinkedDir   = testutils.AbsFilePath(t, "/usr/lib")
		originalSubDir      = testutils.AbsFilePath(t, "/usr/lib/systemd/system")
		originalFilePath    = testutils.AbsFilePath(t, "/usr/lib/systemd/system/test.txt")
		originalFileContent = "test_content"
		symlinkDir          = testutils.AbsFilePath(t, "/lib")
		symlinkFilePath     = testutils.AbsFilePath(t, "/lib/systemd/system/test.txt")
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir) // create absolute symlink
	createFile(t, base, originalFilePath, originalFileContent)

	resolvedPath, found, err := resolvePathWithFound(base, symlinkFilePath)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, filepath.FromSlash(originalFilePath), resolvedPath)
}

func TestUtils_ResolvePathWithRelativeSymlink(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)

	var (
		originalLinkedDir   = testutils.AbsFilePath(t, "/usr/lib")
		originalSubDir      = filepath.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = filepath.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = testutils.AbsFilePath(t, "/lib")
		symlinkSubDir       = filepath.Join(symlinkDir, "/systemd/system")
		symlinkFilePath     = filepath.Join(symlinkSubDir, "test.txt")
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

func TestUtils_ResolveFilePathWithRelativeSymlink(t *testing.T) {
	t.Parallel()

	_, base, _, _ := NewTestBackupFS(t)

	var (
		originalSubDir      = testutils.AbsFilePath(t, "/usr/lib/systemd/system")
		originalFilePath    = testutils.AbsFilePath(t, "/usr/lib/systemd/system/test.txt")
		originalFileContent = "test_content"
		symlinkFile         = testutils.AbsFilePath(t, "/usr/lib/linked_file")
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
	require.Equal(t, originalFilePath, resolvedPath)
}

func currentVolumePrefix() string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.VolumeName(pwd) + separator
}

func currentVolumeDriverLetter() string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(filepath.VolumeName(pwd), ":")
}

func TestUtils_IterateDirTreeAbsolute(t *testing.T) {
	volumePrefix := currentVolumePrefix()
	filePath := filepath.Join(volumePrefix, "a", "b", "c", "d", "test.txt")

	parts := make([]string, 0, 6)
	_, err := IterateDirTree(filePath, func(s string) (proceed bool, err error) {
		parts = append(parts, s)
		return true, nil
	})
	require.NoError(t, err)

	expected := []string{
		volumePrefix,
		filepath.Join(volumePrefix, "a"),
		filepath.Join(volumePrefix, "a", "b"),
		filepath.Join(volumePrefix, "a", "b", "c"),
		filepath.Join(volumePrefix, "a", "b", "c", "d"),
		filepath.Join(volumePrefix, "a", "b", "c", "d", "test.txt"),
	}
	require.Equal(t, expected, parts)
}

func TestUtils_IterateDirTreeRelative(t *testing.T) {
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

func TestUtils_IterateDirTreeEmpty(t *testing.T) {
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
