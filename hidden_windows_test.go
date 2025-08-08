package backupfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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

	// Create the hidden filesystem - use full absolute Windows path with volume prefix
	// This tests HiddenFS ability to handle paths like C:\backup\hidden
	hfs, err := NewHiddenFS(baseFS, absHiddenDir)
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

	// Create hidden filesystem using full absolute Windows path with volume prefix
	// This tests the scenario like C:\Windows\System32\config
	hfs, err := NewHiddenFS(baseFS, absSystemDir)
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

	// Create temporary directory
	tempDir := CallerPathTmp()

	// Setup OSFS
	osfs := NewOSFS()

	baseFS, err := NewPrefixFS(osfs, tempDir)
	assert.NoError(t, err)

	// Define relative Windows paths
	relHiddenDir := "/system/hidden"
	relVisibleDir := "/system/visible"

	// Convert to absolute Windows paths
	absHiddenDir, err := filepath.Abs(filepath.FromSlash(relHiddenDir))
	assert.NoError(t, err)

	absVisibleDir, err := filepath.Abs(filepath.FromSlash(relVisibleDir))
	assert.NoError(t, err)

	// Create HiddenFS using full absolute Windows path with volume prefix
	// This tests symlink operations with paths like C:\system\hidden
	hfs, err := NewHiddenFS(baseFS, absHiddenDir)
	assert.NoError(t, err)

	// Setup directories using absolute paths
	err = baseFS.MkdirAll(absHiddenDir, 0775)
	assert.NoError(t, err)

	err = baseFS.MkdirAll(absVisibleDir, 0775)
	assert.NoError(t, err)

	// Create files
	hiddenFile := filepath.Join(absHiddenDir, "secret.txt")
	visibleFile := filepath.Join(absVisibleDir, "public.txt")

	file, err := baseFS.Create(hiddenFile)
	assert.NoError(t, err)
	err = file.Close()
	assert.NoError(t, err)

	file, err = baseFS.Create(visibleFile)
	assert.NoError(t, err)
	err = file.Close()
	assert.NoError(t, err)

	// Test symlink operations
	symlinkPath := filepath.Join(absVisibleDir, "link.txt")

	// Should not be able to create symlink to hidden file
	err = hfs.Symlink(hiddenFile, symlinkPath)
	assert.ErrorIs(t, err, os.ErrPermission)

	// Should be able to create symlink to visible file using relative path
	validSymlinkPath := symlinkPath + "-valid"
	// Remove any existing symlink from previous test runs
	_ = hfs.Remove(validSymlinkPath) // Ignore error if file doesn't exist
	
	err = hfs.Symlink("public.txt", validSymlinkPath)
	assert.NoError(t, err)

	// Verify symlink exists
	_, err = hfs.Lstat(validSymlinkPath)
	assert.NoError(t, err)

	// Clean up created symlink after test
	err = hfs.Remove(validSymlinkPath)
	assert.NoError(t, err)
}

// TestHiddenFS_WindowsVolumePathIssue tests the specific issue with Windows volume paths
// This reproduces the error: "failed to resolve path: C:\\DBA: lstat C:: lstat C:: hidden check failed: is_hidden C:.: Rel: can't make C:. relative to C:\\DBA\\agent\\runtime\\backups\\f7a406eb-40d6-482d-9f29-2e35d85e4b5b.json"
func TestHiddenFS_WindowsVolumePathIssue(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	// Create temporary directory
	tempDir := CallerPathTmp()

	// Create base OSFS
	osfs := NewOSFS()
	baseFS, err := NewPrefixFS(osfs, tempDir)
	require.NoError(err)

	// Simulate the problematic path scenario
	relDBADir := "/DBA"
	relBackupDir := "/DBA/agent/runtime/backups"
	backupFile := "f7a406eb-40d6-482d-9f29-2e35d85e4b5b.json"

	// Convert to absolute Windows paths - this will create paths like C:\DBA
	absDBADir, err := filepath.Abs(filepath.FromSlash(relDBADir))
	require.NoError(err)

	absBackupDir, err := filepath.Abs(filepath.FromSlash(relBackupDir))
	require.NoError(err)

	// Create HiddenFS with the backup directory as hidden - using full absolute path with volume prefix
	// This should handle C:\DBA\agent\runtime\backups without the "can't make C:. relative" error
	hfs, err := NewHiddenFS(baseFS, absBackupDir)
	require.NoError(err)

	// Setup directory structure
	err = baseFS.MkdirAll(absBackupDir, 0775)
	require.NoError(err)

	// Create the backup file that should be hidden
	backupFilePath := filepath.Join(absBackupDir, backupFile)
	file, err := baseFS.Create(backupFilePath)
	require.NoError(err)
	_, err = file.Write([]byte("backup data"))
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	// Test Lstat on root "/" - this was part of the original error scenario
	_, err = hfs.Lstat("/")
	if err != nil && !isNotFoundError(err) {
		require.NoError(err, "Lstat on root should not fail with volume path errors")
	}

	// Test operations on the DBA directory (should be visible)
	_, err = hfs.Stat(absDBADir)
	require.NoError(err, "DBA directory should be visible")

	// Test that the backup directory is hidden
	_, err = hfs.Stat(absBackupDir)
	require.ErrorIs(err, os.ErrNotExist, "Backup directory should be hidden")

	// Test that the backup file is hidden
	_, err = hfs.Open(backupFilePath)
	require.ErrorIs(err, os.ErrNotExist, "Backup file should be hidden")

	// Create a visible file in the DBA directory
	visibleFile := filepath.Join(absDBADir, "visible.txt")
	file, err = hfs.Create(visibleFile)
	require.NoError(err)
	err = file.Close()
	require.NoError(err)

	// Verify the visible file exists
	_, err = hfs.Stat(visibleFile)
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
