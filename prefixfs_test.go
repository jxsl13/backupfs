package backupfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrefixFSRemoveAllSymlinksBehavior tests the behavior of os.RemoveAll and PrefixFS.RemoveAll
// for symlinks and compares the resulting directory content to ensure they are identical.
func TestPrefixFSRemoveAllSymlinksBehavior(t *testing.T) {
	// Create temporary directories for testing using the standard pattern
	rootPath := CallerPathTmp()

	t.Parallel()

	testCases := []struct {
		name         string
		setupFunc    func(t *testing.T, osDir, prefixDir string)
		removeTarget string
	}{
		{
			name: "remove_directory_with_symlink_to_file",
			setupFunc: func(t *testing.T, osSetupDir, prefixSetupDir string) {
				// we compare the removal of os.RemoveAll with osDir as base directory
				// as well as PrefixFS.RemoveAll with prefixDir in order to compare the behavior of both implementations

				for _, basePath := range []string{osSetupDir, prefixSetupDir} {
					// Create a target file
					targetFile := filepath.Join(basePath, "target.txt")
					require.NoError(t, os.WriteFile(targetFile, []byte("test content"), 0644))

					// Create a directory with a symlink to the file
					testDir := filepath.Join(basePath, "testdir")
					require.NoError(t, os.Mkdir(testDir, 0755))
					symlinkPath := filepath.Join(testDir, "link_to_file")
					require.NoError(t, os.Symlink(targetFile, symlinkPath))
				}

			},
			removeTarget: "testdir",
		},
		{
			name: "remove_directory_with_symlink_to_directory",
			setupFunc: func(t *testing.T, osSetupDir, prefixSetupDir string) {

				for _, basePath := range []string{osSetupDir, prefixSetupDir} {
					// Create a target directory
					targetDir := filepath.Join(basePath, "targetdir")
					require.NoError(t, os.Mkdir(targetDir, 0755))
					require.NoError(t, os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("content"), 0644))

					// Create a directory with a symlink to the directory
					testDir := filepath.Join(basePath, "testdir")
					require.NoError(t, os.Mkdir(testDir, 0755))
					symlinkPath := filepath.Join(testDir, "link_to_dir")
					require.NoError(t, os.Symlink(targetDir, symlinkPath))

				}

			},
			removeTarget: "testdir",
		},
		{
			name: "remove_symlink_directly",
			setupFunc: func(t *testing.T, osSetupDir, prefixSetupDir string) {
				for _, basePath := range []string{osSetupDir, prefixSetupDir} {
					// Create a target file
					targetFile := filepath.Join(basePath, "target.txt")
					require.NoError(t, os.WriteFile(targetFile, []byte("test content"), 0644))

					// Create a symlink to the file
					symlinkPath := filepath.Join(basePath, "link_to_file")
					require.NoError(t, os.Symlink(targetFile, symlinkPath))
				}
			},
			removeTarget: "link_to_file",
		},
		{
			name: "remove_directory_with_nested_symlinks",
			setupFunc: func(t *testing.T, osSetupDir, prefixSetupDir string) {
				for _, basePath := range []string{osSetupDir, prefixSetupDir} {
					// Create target files and directories
					targetFile := filepath.Join(basePath, "target.txt")
					require.NoError(t, os.WriteFile(targetFile, []byte("test content"), 0644))

					targetDir := filepath.Join(basePath, "targetdir")
					require.NoError(t, os.Mkdir(targetDir, 0755))
					require.NoError(t, os.WriteFile(filepath.Join(targetDir, "nested.txt"), []byte("nested content"), 0644))

					// Create a directory structure with nested symlinks
					testDir := filepath.Join(basePath, "testdir")
					require.NoError(t, os.Mkdir(testDir, 0755))
					subDir := filepath.Join(testDir, "subdir")
					require.NoError(t, os.Mkdir(subDir, 0755))

					// Symlinks at different levels
					require.NoError(t, os.Symlink(targetFile, filepath.Join(testDir, "link1")))
					require.NoError(t, os.Symlink(targetDir, filepath.Join(testDir, "link2")))
					require.NoError(t, os.Symlink(targetFile, filepath.Join(subDir, "link3")))
				}

			},
			removeTarget: "testdir",
		},
	}

	for _, tc := range testCases {
		tc := tc // capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			//t.Parallel()

			rootPath := filepath.Join(rootPath, tc.name)
			osFS := NewOSFS()

			// Create a unique base directory for this test
			testBasePath, err := TempDir(osFS, rootPath, fmt.Sprintf("%s-", time.Now().Format("2006-01-02_15-04-05.000")))
			require.NoError(t, err)

			// Create subdirectories for OS and PrefixFS testing
			osSetupDir := filepath.Join(testBasePath, "ostest")
			require.NoError(t, os.MkdirAll(osSetupDir, 0755))

			prefixTestDir := filepath.Join(testBasePath, "prefixtest")
			prefixSetupDir := filepath.Join(prefixTestDir, currentVolumeDriverLetter())
			require.NoError(t, os.MkdirAll(prefixSetupDir, 0755))

			// Setup the test scenario using the OS filesystem directly
			tc.setupFunc(t, osSetupDir, prefixSetupDir)

			prefixFS, err := NewPrefixFS(NewOSFS(), prefixTestDir)
			require.NoError(t, err)

			// Get initial directory contents before removal
			osContentsBefore, err := newFSState(osFS, osSetupDir, true)
			require.NoError(t, err)
			prefixContentsBefore, err := newFSState(prefixFS, currentVolumePrefix(), true) // root directory
			require.NoError(t, err)

			// Verify both directories have the same initial content
			require.Equal(t, osContentsBefore, prefixContentsBefore, "Initial directory contents should be identical")

			// Perform removal with OSFS.RemoveAll
			osErr := osFS.RemoveAll(filepath.Join(osSetupDir, tc.removeTarget))
			require.NoError(t, err)
			// Perform removal with PrefixFS.RemoveAll
			prefixErr := prefixFS.RemoveAll(filepath.Join(currentVolumePrefix(), tc.removeTarget))

			// Compare errors (both should succeed or fail in the same way)
			if osErr != nil {
				assert.Error(t, prefixErr, "PrefixFS.RemoveAll should fail if OSFS.RemoveAll fails")
			} else {
				assert.NoError(t, prefixErr, "PrefixFS.RemoveAll should succeed if OSFS.RemoveAll succeeds")
			}

			// Get directory contents after removal
			osContentsAfter, err := newFSState(osFS, osSetupDir, true)
			require.NoError(t, err)
			prefixContentsAfter, err := newFSState(prefixFS, currentVolumePrefix(), true)
			require.NoError(t, err)

			// Compare final directory contents
			assert.Equal(t, osContentsAfter, prefixContentsAfter, "Directory contents should be identical after RemoveAll operations")

			t.Logf("Initial contents: %d items", len(osContentsBefore))
			t.Logf("Final contents: %d items", len(osContentsAfter))
			t.Logf("Removed items: %d", len(osContentsBefore)-len(osContentsAfter))
		})
	}
}
