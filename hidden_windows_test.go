package backupfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/assert"
)

// TestHiddenFS_WindowsAbsolutePaths tests HiddenFS functionality with absolute Windows paths
func TestHiddenFS_WindowsAbsolutePaths(t *testing.T) {
	t.Parallel()

	// Create temporary directory for testing
	tempDir := CallerPathTmp()

	// Create base OSFS
	osfs := NewOSFS()

	// Create a prefix filesystem under the temp directory
	baseFS, err := NewPrefixFS(osfs, tempDir)
	if !assert.NoError(t, err) {
		return
	}

	// Define relative Windows paths starting with /
	absHiddenDirParent := testutils.AbsFilePath(t, "/backup")
	absHiddenDir := testutils.AbsFilePath(t, "/backup/hidden")
	hiddenFile := "secret.json"

	// Create the hidden filesystem - use full absolute Windows path with volume prefix
	// This tests HiddenFS ability to handle paths like C:\backup\hidden
	hfs, err := NewHiddenFS(baseFS, absHiddenDir)
	if !assert.NoError(t, err) {
		return
	}

	// Setup the test environment using absolute paths
	err = baseFS.MkdirAll(absHiddenDirParent, 0775)
	if !assert.NoError(t, err) {
		return
	}

	err = baseFS.MkdirAll(absHiddenDir, 0775)
	if !assert.NoError(t, err) {
		return
	}

	// Create a hidden file
	hiddenFilePath := filepath.Join(absHiddenDir, hiddenFile)
	file, err := baseFS.Create(hiddenFilePath)
	if !assert.NoError(t, err) {
		return
	}
	_, err = file.Write([]byte("secret content"))
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

	// Test that the hidden directory appears to not exist from HiddenFS perspective
	_, err = hfs.Stat(absHiddenDir)
	if !assert.ErrorIs(t, err, os.ErrNotExist) {
		return
	}

	// Test that files in the hidden directory cannot be accessed
	_, err = hfs.Open(hiddenFilePath)
	if !assert.ErrorIs(t, err, os.ErrNotExist) {
		return
	}

	// Test that we cannot create files in the hidden directory
	_, err = hfs.Create(filepath.Join(absHiddenDir, "new_file.txt"))
	if !assert.ErrorIs(t, err, os.ErrPermission) {
		return
	}

	// Test that we can create files outside the hidden directory
	visibleFile := filepath.Join(absHiddenDirParent, "visible.txt")
	file, err = hfs.Create(visibleFile)
	if !assert.NoError(t, err) {
		return
	}
	_, err = file.Write([]byte("visible content"))
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

	// Verify the visible file exists
	_, err = hfs.Stat(visibleFile)
	if !assert.NoError(t, err) {
		return
	}

	// Test listing directory contents - hidden directory should not appear
	entries, err := readDir(hfs, absHiddenDirParent)
	if !assert.NoError(t, err) {
		return
	}

	// Should only see the visible file, not the hidden directory
	if !assert.Len(t, entries, 1) {
		return
	}
	if !assert.Equal(t, "visible.txt", entries[0].Name()) {
		return
	}
}

// TestHiddenFS_WindowsSystemPaths tests HiddenFS with Windows system-like absolute paths
func TestHiddenFS_WindowsSystemPaths(t *testing.T) {
	t.Parallel()

	// Create a temporary directory
	tempDir := CallerPathTmp()

	// Create base OSFS
	osfs := NewOSFS()

	// Create prefix filesystem
	baseFS, err := NewPrefixFS(osfs, tempDir)
	if !assert.NoError(t, err) {
		return
	}

	// Use Windows system-like relative paths
	absSystemDir := testutils.AbsFilePath(t, "/Windows/System32/config")
	systemFile := "system.dat"

	// Create hidden filesystem using full absolute Windows path with volume prefix
	// This tests the scenario like C:\Windows\System32\config
	hfs, err := NewHiddenFS(baseFS, absSystemDir)
	if !assert.NoError(t, err) {
		return
	}

	// Setup test directory structure using absolute path
	err = baseFS.MkdirAll(absSystemDir, 0775)
	if !assert.NoError(t, err) {
		return
	}

	// Create hidden file
	hiddenFilePath := filepath.Join(absSystemDir, systemFile)
	file, err := baseFS.Create(hiddenFilePath)
	if !assert.NoError(t, err) {
		return
	}
	_, err = file.Write([]byte("system data"))
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

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
	if !assert.NoError(t, err) {
		return
	}

	absHiddenDir := testutils.AbsFilePath(t, "/system/hidden")
	absVisibleDir := testutils.AbsFilePath(t, "/system/visible")

	// Create HiddenFS using full absolute Windows path with volume prefix
	// This tests symlink operations with paths like C:\system\hidden
	hfs, err := NewHiddenFS(baseFS, absHiddenDir)
	if !assert.NoError(t, err) {
		return
	}

	// Setup directories using absolute paths
	err = baseFS.MkdirAll(absHiddenDir, 0775)
	if !assert.NoError(t, err) {
		return
	}

	err = baseFS.MkdirAll(absVisibleDir, 0775)
	if !assert.NoError(t, err) {
		return
	}

	// Create files
	hiddenFile := filepath.Join(absHiddenDir, "secret.txt")
	visibleFile := filepath.Join(absVisibleDir, "public.txt")

	file, err := baseFS.Create(hiddenFile)
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

	file, err = baseFS.Create(visibleFile)
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

	// Test symlink operations
	symlinkPath := filepath.Join(absVisibleDir, "link.txt")

	// Should not be able to create symlink to hidden file
	err = hfs.Symlink(hiddenFile, symlinkPath)
	if !assert.ErrorIs(t, err, os.ErrPermission) {
		return
	}

	// Should be able to create symlink to visible file using relative path
	validSymlinkPath := symlinkPath + "-valid"
	// Remove any existing symlink from previous test runs
	_ = hfs.Remove(validSymlinkPath) // Ignore error if file doesn't exist

	err = hfs.Symlink("public.txt", validSymlinkPath)
	if !assert.NoError(t, err) {
		return
	}

	// Verify symlink exists
	_, err = hfs.Lstat(validSymlinkPath)
	if !assert.NoError(t, err) {
		return
	}

	// Clean up created symlink after test
	err = hfs.Remove(validSymlinkPath)
	if !assert.NoError(t, err) {
		return
	}
}

// TestHiddenFS_WindowsVolumePathIssue tests the specific issue with Windows volume paths
// This reproduces the error: "failed to resolve path: C:\\DBA: lstat C:: lstat C:: hidden check failed: is_hidden C:.: Rel: can't make C:. relative to C:\\DBA\\agent\\runtime\\backups\\f7a406eb-40d6-482d-9f29-2e35d85e4b5b.json"
func TestHiddenFS_WindowsVolumePathIssue(t *testing.T) {
	t.Parallel()

	// Create temporary directory
	tempDir := CallerPathTmp()

	// Create base OSFS
	osfs := NewOSFS()
	baseFS, err := NewPrefixFS(osfs, tempDir)
	if !assert.NoError(t, err) {
		return
	}

	// Simulate the problematic path scenario
	absDBADir := testutils.AbsFilePath(t, "/DBA")
	absBackupDir := testutils.AbsFilePath(t, "/DBA/agent/runtime/backups")
	backupFile := "f7a406eb-40d6-482d-9f29-2e35d85e4b5b.json"

	// Create HiddenFS with the backup directory as hidden - using full absolute path with volume prefix
	// This should handle C:\DBA\agent\runtime\backups without the "can't make C:. relative" error
	hfs, err := NewHiddenFS(baseFS, absBackupDir)
	if !assert.NoError(t, err) {
		return
	}

	// Setup directory structure
	err = baseFS.MkdirAll(absBackupDir, 0775)
	if !assert.NoError(t, err) {
		return
	}

	// Create the backup file that should be hidden
	backupFilePath := filepath.Join(absBackupDir, backupFile)
	file, err := baseFS.Create(backupFilePath)
	if !assert.NoError(t, err) {
		return
	}
	_, err = file.Write([]byte("backup data"))
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

	// Test Lstat on root "/" - this was part of the original error scenario
	_, err = hfs.Lstat("/")
	if err != nil && !isNotFoundError(err) {
		if !assert.NoError(t, err, "Lstat on root should not fail with volume path errors") {
			return
		}
	}

	// Test operations on the DBA directory (should be visible)
	_, err = hfs.Stat(absDBADir)
	if !assert.NoError(t, err, "DBA directory should be visible") {
		return
	}

	// Test that the backup directory is hidden
	_, err = hfs.Stat(absBackupDir)
	if !assert.ErrorIs(t, err, os.ErrNotExist, "Backup directory should be hidden") {
		return
	}

	// Test that the backup file is hidden
	_, err = hfs.Open(backupFilePath)
	if !assert.ErrorIs(t, err, os.ErrNotExist, "Backup file should be hidden") {
		return
	}

	// Create a visible file in the DBA directory
	visibleFile := filepath.Join(absDBADir, "visible.txt")
	file, err = hfs.Create(visibleFile)
	if !assert.NoError(t, err) {
		return
	}
	err = file.Close()
	if !assert.NoError(t, err) {
		return
	}

	// Verify the visible file exists
	_, err = hfs.Stat(visibleFile)
	if !assert.NoError(t, err) {
		return
	}
}

// helper function to test hidden operations
func testHiddenOperations(t *testing.T, hfs *HiddenFS, baseFS FS, hiddenDir, hiddenFile string) {
	// Hidden directory should not be visible
	_, err := hfs.Stat(hiddenDir)
	if !assert.ErrorIs(t, err, os.ErrNotExist) {
		return
	}

	// Hidden file should not be accessible
	_, err = hfs.Open(hiddenFile)
	if !assert.ErrorIs(t, err, os.ErrNotExist) {
		return
	}

	// Cannot create files in hidden directory
	_, err = hfs.Create(filepath.Join(hiddenDir, "new.txt"))
	if !assert.ErrorIs(t, err, os.ErrPermission) {
		return
	}

	// Cannot remove hidden directory
	err = hfs.Remove(hiddenDir)
	if !assert.ErrorIs(t, err, os.ErrNotExist) {
		return
	}

	// Cannot make directories in hidden path
	err = hfs.Mkdir(filepath.Join(hiddenDir, "subdir"), 0775)
	if !assert.ErrorIs(t, err, os.ErrPermission) {
		return
	}

	// Verify files still exist in base filesystem
	_, err = baseFS.Stat(hiddenFile)
	if !assert.NoError(t, err) {
		return
	}
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
