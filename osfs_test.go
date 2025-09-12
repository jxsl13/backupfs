package backupfs

import (
	"testing"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestOSFS_RemoveFileSymlink(t *testing.T) {
	t.Parallel()

	osfs := NewOSFS()
	rootPath, err := TempDir(osfs, FuncPathTmp(), TimesStamp())
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
	rootPath, err := TempDir(osfs, FuncPathTmp())
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

// TestOSFS_RemoveDirectoryWithSymlinks tests the behavior of the OS filesystem
// when removing a directory that contains symlinks pointing to files and directories
// outside the removed directory. This test validates our assumptions used in
// TestBackupFS_RemoveFileSymlink about how the underlying OS handles symlink removal.
//
// This test verifies that:
// 1. Removing a directory containing symlinks only removes the symlinks themselves
// 2. Target files/directories remain intact when symlinks pointing to them are removed
// 3. The OS filesystem behavior matches our expectations in BackupFS tests
//
// Test Setup (mirrors TestBackupFS_RemoveFileSymlink):
// Creates the same directory structure as the BackupFS test:
//   - /dir1/ - Contains symlinks that will be removed
//     ├── link_to_file -> /dir2/dir3/file.txt (symlink to file)
//     └── link_to_dir -> /dir2/dir3/ (symlink to directory)
//   - /dir2/dir3/ - Contains target resources
//     └── file.txt - Target file with test content
//
// Test Flow:
// 1. Setup: Create directory structure, target file, and symlinks
// 2. Verify: Confirm all symlinks exist and point to correct targets
// 3. Remove: Delete /dir1/ (containing symlinks) using os.RemoveAll()
// 4. Verify: Ensure symlinks are removed but targets remain intact
func TestOSFS_RemoveDirectoryWithSymlinks(t *testing.T) {
	t.Parallel()

	// Initialize OS filesystem and create temporary directory
	osfs := NewOSFS()
	rootPath, err := TempDir(osfs, testutils.FuncName())
	require.NoError(t, err)

	// Setup the root directory with proper permissions
	mkdirAll(t, osfs, rootPath, 0o700)

	// Define directory structure (mirrors TestBackupFS_RemoveFileSymlink)
	// Directory structure layout:
	// /dir1/link_to_file -> /dir2/dir3/file.txt (file symlink)
	// /dir1/link_to_dir -> /dir2/dir3 (directory symlink)
	const (
		dir1        = "/dir1"                           // Directory containing symlinks (to be removed)
		dir3        = "/dir2/dir3"                      // Target directory for symlinks
		targetFile  = "/dir2/dir3/file.txt"             // Target file for file symlink
		linkToFile  = "/dir1/link_to_file"              // Symlink pointing to targetFile
		linkToDir   = "/dir1/link_to_dir"               // Symlink pointing to dir3
		fileContent = "test content for symlink target" // Content for target file
	)

	// === SETUP PHASE ===
	// Create the required directory structure in the OS filesystem
	mkdirAll(t, osfs, dir1, 0o755) // Create directory that will contain symlinks
	mkdirAll(t, osfs, dir3, 0o755) // Create target directory for symlinks

	// Create the target file that one of the symlinks will point to
	createFile(t, osfs, targetFile, fileContent)
	fileMustContainText(t, osfs, targetFile, fileContent)

	// Create symlinks in dir1 pointing to targets in dir2/dir3
	createSymlink(t, osfs, targetFile, linkToFile) // File symlink: dir1/link_to_file -> dir2/dir3/file.txt
	createSymlink(t, osfs, dir3, linkToDir)        // Directory symlink: dir1/link_to_dir -> dir2/dir3

	// === VERIFICATION PHASE ===
	// Verify that symlinks were created successfully and point to correct targets
	symlinkMustExist(t, osfs, linkToFile)
	symlinkMustExist(t, osfs, linkToDir)
	symlinkMustExistWithTragetPath(t, osfs, linkToFile, targetFile)
	symlinkMustExistWithTragetPath(t, osfs, linkToDir, dir3)

	// === REMOVAL PHASE ===
	// Remove dir1 (containing both symlinks) using OS RemoveAll
	// This is the critical operation we're testing to ensure it behaves as expected
	err = osfs.RemoveAll(dir1)
	require.NoError(t, err)

	// === POST-REMOVAL VERIFICATION PHASE ===
	// Verify that dir1 and its symlinks are completely removed from the filesystem
	mustNotExist(t, osfs, dir1)        // Directory should not exist
	mustNotLExist(t, osfs, linkToFile) // File symlink should not exist
	mustNotLExist(t, osfs, linkToDir)  // Directory symlink should not exist

	// CRITICAL VERIFICATION: Verify that symlink targets remain intact
	// This is the key assumption that BackupFS relies on - removing symlinks
	// should NOT affect the files/directories they point to
	mustExist(t, osfs, targetFile)                        // Target file should still exist
	mustExist(t, osfs, dir3)                              // Target directory should still exist
	fileMustContainText(t, osfs, targetFile, fileContent) // Target file content should be unchanged

	// Additional verification: Ensure target directory structure is intact
	mustExist(t, osfs, "/dir2") // Parent directory of target should still exist
}

// TestOSFS_RemoveIndividualSymlinks tests the behavior of the OS filesystem
// when removing individual symlinks using Remove() (not RemoveAll()).
// This test validates our assumptions used in BackupFS.Remove() operations
// about how the underlying OS handles individual symlink removal.
//
// This test verifies that:
// 1. Removing individual symlinks only removes the symlink itself
// 2. Target files/directories remain intact when symlinks pointing to them are removed
// 3. The OS filesystem behavior matches our expectations in BackupFS tests
//
// Test Setup:
// Creates a directory structure with individual symlinks:
//   - /dir2/dir3/file.txt - Target file
//   - /link_to_file -> /dir2/dir3/file.txt (file symlink to be removed)
//   - /link_to_dir -> /dir2/dir3/ (directory symlink to be removed)
//
// Test Flow:
// 1. Setup: Create target file and individual symlinks
// 2. Verify: Confirm all symlinks exist and point to correct targets
// 3. Remove: Delete individual symlinks using os.Remove()
// 4. Verify: Ensure symlinks are removed but targets remain intact
func TestOSFS_RemoveIndividualSymlinks(t *testing.T) {
	t.Parallel()

	// Initialize OS filesystem and create temporary directory

	osfs := NewOSFS()
	rootPath, err := TempDir(osfs, FuncPathTmp(), TimesStamp())
	require.NoError(t, err)

	// Setup the root directory with proper permissions
	mkdirAll(t, osfs, rootPath, 0o700)

	// Define paths for individual symlink removal test
	const (
		dir3        = "/dir2/dir3"                                 // Target directory
		targetFile  = "/dir2/dir3/file.txt"                        // Target file
		linkToFile  = "/link_to_file"                              // File symlink (to be removed individually)
		linkToDir   = "/link_to_dir"                               // Directory symlink (to be removed individually)
		fileContent = "test content for individual symlink target" // Content for target file
	)

	// === SETUP PHASE ===
	// Create target directory and file
	mkdirAll(t, osfs, dir3, 0o755)
	createFile(t, osfs, targetFile, fileContent)
	fileMustContainText(t, osfs, targetFile, fileContent)

	// Create individual symlinks pointing to targets
	createSymlink(t, osfs, targetFile, linkToFile) // File symlink: /link_to_file -> /dir2/dir3/file.txt
	createSymlink(t, osfs, dir3, linkToDir)        // Directory symlink: /link_to_dir -> /dir2/dir3

	// === VERIFICATION PHASE ===
	// Verify that symlinks were created successfully and point to correct targets
	symlinkMustExist(t, osfs, linkToFile)
	symlinkMustExist(t, osfs, linkToDir)
	symlinkMustExistWithTragetPath(t, osfs, linkToFile, targetFile)
	symlinkMustExistWithTragetPath(t, osfs, linkToDir, dir3)

	// === REMOVAL PHASE ===
	// Remove individual symlinks using OS Remove (not RemoveAll)
	err = osfs.Remove(linkToFile)
	require.NoError(t, err)

	err = osfs.Remove(linkToDir)
	require.NoError(t, err)

	// === POST-REMOVAL VERIFICATION PHASE ===
	// Verify that individual symlinks are removed
	mustNotLExist(t, osfs, linkToFile) // File symlink should not exist
	mustNotLExist(t, osfs, linkToDir)  // Directory symlink should not exist

	// CRITICAL VERIFICATION: Verify that symlink targets remain intact
	// This is the key assumption that BackupFS relies on - removing symlinks
	// should NOT affect the files/directories they point to
	mustExist(t, osfs, targetFile)                        // Target file should still exist
	mustExist(t, osfs, dir3)                              // Target directory should still exist
	fileMustContainText(t, osfs, targetFile, fileContent) // Target file content should be unchanged

	// Additional verification: Ensure target directory structure is intact
	mustExist(t, osfs, "/dir2") // Parent directory of target should still exist
}
