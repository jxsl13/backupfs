package backupfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHiddenFS_WindowsAbsolutePaths tests HiddenFS functionality with absolute Windows paths
func TestHiddenFS_WindowsAbsolutePaths(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	// Create temporary directory for testing
	tempDir := CallerPathTmp()

	// Create base OSFS
	osfs := NewOSFS()

	// Create a prefix filesystem under the temp directory
	baseFS, err := NewPrefixFS(osfs, tempDir)
	require.NoError(err)

	// Define relative Windows paths starting with /
	relHiddenDirParent := "/backup"
	relHiddenDir := "/backup/hidden"
	hiddenFile := "secret.json"

	// Convert to absolute Windows paths using filepath.Abs and filepath.FromSlash
	absHiddenDirParent, err := filepath.Abs(filepath.FromSlash(relHiddenDirParent))
	require.NoError(err)

	absHiddenDir, err := filepath.Abs(filepath.FromSlash(relHiddenDir))
	require.NoError(err)

	// For HiddenFS, we need to use paths without volume prefix
	hiddenDirForHFS := TrimVolume(absHiddenDir)

	// Create the hidden filesystem - paths must not have volume prefix
	hfs, err := NewHiddenFS(baseFS, hiddenDirForHFS)
	require.NoError(err)

	// Setup the test environment using absolute paths
	err = baseFS.MkdirAll(absHiddenDirParent, 0775)
	require.NoError(err)

	err = baseFS.MkdirAll(absHiddenDir, 0775)
	require.NoError(err)

	// Create a hidden file
	hiddenFilePath := filepath.Join(absHiddenDir, hiddenFile)
	file, err := baseFS.Create(hiddenFilePath)
	require.NoError(err)
	_, err = file.Write([]byte("secret content"))
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	// Test that the hidden directory appears to not exist from HiddenFS perspective
	_, err = hfs.Stat(absHiddenDir)
	require.ErrorIs(err, os.ErrNotExist)

	// Test that files in the hidden directory cannot be accessed
	_, err = hfs.Open(hiddenFilePath)
	require.ErrorIs(err, os.ErrNotExist)

	// Test that we cannot create files in the hidden directory
	_, err = hfs.Create(filepath.Join(absHiddenDir, "new_file.txt"))
	require.ErrorIs(err, os.ErrPermission)

	// Test that we can create files outside the hidden directory
	visibleFile := filepath.Join(absHiddenDirParent, "visible.txt")
	file, err = hfs.Create(visibleFile)
	require.NoError(err)
	_, err = file.Write([]byte("visible content"))
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	// Verify the visible file exists
	_, err = hfs.Stat(visibleFile)
	require.NoError(err)

	// Test listing directory contents - hidden directory should not appear
	entries, err := readDir(hfs, absHiddenDirParent)
	require.NoError(err)

	// Should only see the visible file, not the hidden directory
	require.Len(entries, 1)
	require.Equal("visible.txt", entries[0].Name())
}

// TestHiddenFS_WindowsSystemPaths tests HiddenFS with Windows system-like absolute paths
func TestHiddenFS_WindowsSystemPaths(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	// Create a temporary directory
	tempDir := CallerPathTmp()

	// Create base OSFS
	osfs := NewOSFS()

	// Create prefix filesystem
	baseFS, err := NewPrefixFS(osfs, tempDir)
	require.NoError(err)

	// Use Windows system-like relative paths
	relSystemDir := "/Windows/System32/config"
	systemFile := "system.dat"

	// Convert to absolute Windows path
	absSystemDir, err := filepath.Abs(filepath.FromSlash(relSystemDir))
	require.NoError(err)

	// For HiddenFS, trim the volume prefix
	systemDirForHFS := TrimVolume(absSystemDir)

	// Create hidden filesystem - paths must not have volume prefix
	hfs, err := NewHiddenFS(baseFS, systemDirForHFS)
	require.NoError(err)

	// Setup test directory structure using absolute path
	err = baseFS.MkdirAll(absSystemDir, 0775)
	require.NoError(err)

	// Create hidden file
	hiddenFilePath := filepath.Join(absSystemDir, systemFile)
	file, err := baseFS.Create(hiddenFilePath)
	require.NoError(err)
	_, err = file.Write([]byte("system data"))
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	// Test operations on hidden paths
	testHiddenOperations(t, hfs, baseFS, absSystemDir, hiddenFilePath)
}

// TestHiddenFS_WindowsSymlinkPaths tests symlink operations with Windows absolute paths
func TestHiddenFS_WindowsSymlinkPaths(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	// Create temporary directory
	tempDir := CallerPathTmp()

	// Setup OSFS
	osfs := NewOSFS()

	baseFS, err := NewPrefixFS(osfs, tempDir)
	require.NoError(err)

	// Define relative Windows paths
	relHiddenDir := "/system/hidden"
	relVisibleDir := "/system/visible"

	// Convert to absolute Windows paths
	absHiddenDir, err := filepath.Abs(filepath.FromSlash(relHiddenDir))
	require.NoError(err)

	absVisibleDir, err := filepath.Abs(filepath.FromSlash(relVisibleDir))
	require.NoError(err)

	// For HiddenFS, trim volume prefix
	hiddenDirForHFS := TrimVolume(absHiddenDir)

	// Create HiddenFS
	hfs, err := NewHiddenFS(baseFS, hiddenDirForHFS)
	require.NoError(err)

	// Setup directories using absolute paths
	err = baseFS.MkdirAll(absHiddenDir, 0775)
	require.NoError(err)

	err = baseFS.MkdirAll(absVisibleDir, 0775)
	require.NoError(err)

	// Create files
	hiddenFile := filepath.Join(absHiddenDir, "secret.txt")
	visibleFile := filepath.Join(absVisibleDir, "public.txt")

	file, err := baseFS.Create(hiddenFile)
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	file, err = baseFS.Create(visibleFile)
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	// Test symlink operations
	symlinkPath := filepath.Join(absVisibleDir, "link.txt")

	// Should not be able to create symlink to hidden file
	err = hfs.Symlink(hiddenFile, symlinkPath)
	require.ErrorIs(err, os.ErrPermission)

	// Should be able to create symlink to visible file using relative path
	err = hfs.Symlink("public.txt", symlinkPath+"-valid")
	require.NoError(err)

	// Verify symlink exists
	_, err = hfs.Lstat(symlinkPath + "-valid")
	require.NoError(err)
}

// helper function to test hidden operations
func testHiddenOperations(t *testing.T, hfs *HiddenFS, baseFS FS, hiddenDir, hiddenFile string) {
	require := require.New(t)

	// Hidden directory should not be visible
	_, err := hfs.Stat(hiddenDir)
	require.ErrorIs(err, os.ErrNotExist)

	// Hidden file should not be accessible
	_, err = hfs.Open(hiddenFile)
	require.ErrorIs(err, os.ErrNotExist)

	// Cannot create files in hidden directory
	_, err = hfs.Create(filepath.Join(hiddenDir, "new.txt"))
	require.ErrorIs(err, os.ErrPermission)

	// Cannot remove hidden directory
	err = hfs.Remove(hiddenDir)
	require.ErrorIs(err, os.ErrNotExist)

	// Cannot make directories in hidden path
	err = hfs.Mkdir(filepath.Join(hiddenDir, "subdir"), 0775)
	require.ErrorIs(err, os.ErrPermission)

	// Verify files still exist in base filesystem
	_, err = baseFS.Stat(hiddenFile)
	require.NoError(err)
}

// helper function to read directory contents
func readDir(fsys FS, dirname string) ([]os.DirEntry, error) {
	file, err := fsys.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if dirReader, ok := file.(interface {
		ReadDir(int) ([]os.DirEntry, error)
	}); ok {
		return dirReader.ReadDir(-1)
	}

	// Fallback for filesystems that don't implement ReadDir
	fileInfos, err := file.Readdir(-1)
	if err != nil {
		return nil, err
	}

	entries := make([]os.DirEntry, len(fileInfos))
	for i, fi := range fileInfos {
		entries[i] = &dirEntry{fi}
	}
	return entries, nil
}

// dirEntry implements os.DirEntry for FileInfo
type dirEntry struct {
	info os.FileInfo
}

func (d *dirEntry) Name() string {
	return d.info.Name()
}

func (d *dirEntry) IsDir() bool {
	return d.info.IsDir()
}

func (d *dirEntry) Type() os.FileMode {
	return d.info.Mode().Type()
}

func (d *dirEntry) Info() (os.FileInfo, error) {
	return d.info, nil
}
