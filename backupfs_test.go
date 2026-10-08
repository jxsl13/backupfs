package backupfs

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jxsl13/backupfs/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestBackupFS_Create(t *testing.T) {
	t.Parallel()

	root, base, backup, backupFS := NewTestBackupFS(t)
	defer func() {
		// Clean up by removing the contents, not the root directory itself
		require.NoError(t, root.RemoveAll("/base"))
	}()

	var (
		err                         error
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_content"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)

	absFilePath := testutils.AbsFilePath(t, filePath)

	createFile(t, base, absFilePath, fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	createFile(t, backupFS, filePath, fileContentOverwritten)

	fileMustContainText(t, base, filePath, fileContentOverwritten)
	fileMustContainText(t, backup, filePath, fileContent)

	createFile(t, backupFS, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, backupFS, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, base, filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	fileMustContainText(t, backup, filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	createFile(t, backupFS, newFilePath, fileContent)
	fileMustContainText(t, base, newFilePath, fileContent)
	mustNotExist(t, backup, newFilePath)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_CreateAvoidsDuplicateLstat(t *testing.T) {
	t.Parallel()

	for _, existingFile := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing file", false: "missing file"}[existingFile], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/create/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			if existingFile {
				createFile(t, base, filePath, "original contents")
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for _, contents := range []string{"first write", "second write"} {
				counting.calls[filePath] = 0
				createFile(t, backupFS, filePath, contents)
				require.Equal(t, 1, counting.calls[filePath], "reuse file metadata from path resolution")
				fileMustContainText(t, base, filePath, contents)
				if existingFile {
					fileMustContainText(t, backup, filePath, "original contents")
				} else {
					mustNotExist(t, backup, filePath)
				}
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_RejectsEmptyPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		call func(*BackupFS) error
	}{
		{name: "Create", call: func(backupFS *BackupFS) error {
			file, err := backupFS.Create("")
			if err == nil {
				return file.Close()
			}
			return err
		}},
		{name: "Rename source", call: func(backupFS *BackupFS) error {
			return backupFS.Rename("", "target")
		}},
		{name: "Rename target", call: func(backupFS *BackupFS) error {
			return backupFS.Rename(testutils.AbsFilePath(t, "/"), "")
		}},
		{name: "realPath", call: func(backupFS *BackupFS) error {
			_, err := backupFS.realPath("")
			return err
		}},
		{name: "realPathWithFound", call: func(backupFS *BackupFS) error {
			_, _, err := backupFS.realPathWithFound("")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, backupFS := NewTestBackupFS(t)
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting
			err := test.call(backupFS)
			require.ErrorContains(t, err, "empty file path")
			if test.name != "Rename target" {
				require.Empty(t, counting.calls)
			}
		})
	}
}

func TestBackupFS_CreateNormalizesUncleanPath(t *testing.T) {
	t.Parallel()
	_, base, _, backupFS := NewTestBackupFS(t)
	dir := testutils.AbsFilePath(t, "/test/create")
	mkdirAll(t, base, dir, 0755)
	unclean := dir + separator + "unused" + separator + ".." + separator + "." + separator + "file"
	file, err := backupFS.Create(unclean)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	fileMustContainText(t, base, filepath.Join(dir, "file"), "")
}

func TestBackupFS_Name(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	_, _, _, backupFS := NewTestBackupFS(t)

	require.Equal(backupFS.Name(), "BackupFS")
}

func TestBackupFS_OpenFile(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		filePath                    = "/test/01/test_01.txt"
		fileContent                 = "test_content"
		fileContentOverwritten      = fileContent + "_overwritten"
		fileContentOverwrittenAgain = fileContentOverwritten + "_again"
	)
	openFile(t, base, filePath, fileContent, 0755)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	openFile(t, backupFS, filePath, fileContentOverwritten, 1755)

	fileMustContainText(t, base, filePath, fileContentOverwritten)
	fileMustContainText(t, backup, filePath, fileContent)

	openFile(t, backupFS, filePath, fileContentOverwrittenAgain, 0766)
	fileMustContainText(t, backupFS, filePath, fileContentOverwrittenAgain)
	fileMustContainText(t, base, filePath, fileContentOverwrittenAgain)
	// the backed up file should still have the same state as the first initial file
	fileMustContainText(t, backup, filePath, fileContent)

	var (
		newFilePath = "/test/02/test_02.txt"
	)

	openFile(t, backupFS, newFilePath, fileContent, 0755)
	fileMustContainText(t, base, newFilePath, fileContent)
	mustNotExist(t, backup, newFilePath)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_OpenFileAvoidsDuplicateLstat(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		flags    int
		existing bool
	}{
		{name: "truncate existing", flags: os.O_WRONLY | os.O_TRUNC, existing: true},
		{name: "create missing", flags: os.O_RDWR | os.O_CREATE | os.O_TRUNC},
		{name: "append existing", flags: os.O_WRONLY | os.O_APPEND, existing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/open/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			expected := ""
			if test.existing {
				expected = "original contents"
				createFile(t, base, filePath, expected)
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for _, contents := range []string{"first write", "second write"} {
				counting.calls[filePath] = 0
				file, err := backupFS.OpenFile(filePath, test.flags, 0600)
				require.NoError(t, err)
				written, err := file.WriteString(contents)
				require.NoError(t, err)
				require.Equal(t, len(contents), written)
				require.NoError(t, file.Close())
				require.Equal(t, 1, counting.calls[filePath], "reuse file metadata from path resolution")
				if test.flags&os.O_APPEND != 0 {
					expected += contents
				} else {
					expected = contents
				}
				fileMustContainText(t, base, filePath, expected)
				if test.existing {
					fileMustContainText(t, backup, filePath, "original contents")
				} else {
					mustNotExist(t, backup, filePath)
				}
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_OpenFileReadOnlyBypassesSnapshots(t *testing.T) {
	t.Parallel()
	_, base, backup, backupFS := NewTestBackupFS(t)
	filePath := testutils.AbsFilePath(t, "/test/open/file")
	createFile(t, base, filePath, "original contents")
	counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
	backupFS.base = counting
	file, err := backupFS.OpenFile(filePath, os.O_RDONLY, 0)
	require.NoError(t, err)
	contents, err := io.ReadAll(file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.Equal(t, "original contents", string(contents))
	require.Empty(t, counting.calls)
	require.Empty(t, backupFS.Map())
	mustNotExist(t, backup, filePath)
}

func TestBackupFS_Remove(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		filePath    = "/test/01/test_01.txt"
		fileContent = "test_content"
	)
	createFile(t, base, filePath, fileContent)
	fileMustContainText(t, base, filePath, fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	removeFile(t, backupFS, filePath)
	mustNotExist(t, backupFS, filePath)

	mustNotExist(t, base, filePath)

	mustExist(t, backup, filePath)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_RemoveAll(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = testutils.AbsFilePath(t, "/test")
		fileDir     = testutils.AbsFilePath(t, "/test/001")
		fileDir2    = testutils.AbsFilePath(t, "/test/0/2")
		symlinkDir  = testutils.AbsFilePath(t, "/test/sym")
		fileContent = "test_content"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)
	mkdirAll(t, base, symlinkDir, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	// symlink pointing at random location that doesnot exist
	createSymlink(t, base, filepath.Join(fileDir, "/test00.txt"), filepath.Join(symlinkDir, "/link"))
	createSymlink(t, base, filepath.Join(fileDir, "/test00.txt"), filepath.Join(symlinkDir, "/link2"))

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	removeAll(t, backupFS, filepath.Join(symlinkDir, "/link"))
	removeAll(t, backupFS, filepath.Join(symlinkDir, "/fileDoesNotExistAndShouldNotThrowAnErrorWhenUsedInRemoveAll"))
	mustNotLExist(t, backupFS, filepath.Join(symlinkDir, "/link"))

	// remove /test dir
	removeAll(t, backupFS, fileDirRoot)
	mustNotExist(t, backupFS, fileDirRoot)

	// deleted from base file system
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))
	mustNotExist(t, base, filepath.Join(fileDir2, "/test03.txt"))
	mustNotExist(t, base, filepath.Join(fileDir2, "/test04.txt"))

	// link2 is a symlink in one of the sub folders in the
	// directory that is being removed with all of its content
	mustNotLExist(t, backupFS, filepath.Join(symlinkDir, "/link2"))

	mustNotExist(t, base, fileDirRoot)
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, fileDir2)

	// must exist in bakcup
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test01.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test02.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	mustExist(t, backup, fileDir)
	mustExist(t, backup, fileDir2)

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(t, err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_RemoveAllReusesObservedMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		kind     string
		maxLstat int
	}{
		{name: "regular file", kind: "file", maxLstat: 1},
		{name: "empty directory", kind: "directory", maxLstat: 2},
		{name: "nested tree", kind: "tree", maxLstat: 2},
		{name: "missing path", kind: "missing", maxLstat: 1},
		{name: "final symlink", kind: "symlink", maxLstat: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.kind == "symlink" && runtime.GOOS == "windows" {
				t.Skip("creating symlinks requires privileges on Windows")
			}
			_, base, backup, backupFS := NewTestBackupFS(t)
			parent := testutils.AbsFilePath(t, "/test/removeall")
			mkdirAll(t, base, parent, 0755)
			target := filepath.Join(parent, "target")
			untouched := filepath.Join(parent, "untouched")
			createFile(t, base, untouched, "untouched contents")
			var files []string
			switch test.kind {
			case "file":
				createFile(t, base, target, "original contents")
			case "directory":
				mkdirAll(t, base, target, 0700)
			case "tree":
				files = []string{filepath.Join(target, "first"), filepath.Join(target, "nested", "second")}
				for _, filePath := range files {
					createFile(t, base, filePath, "original contents")
				}
			case "symlink":
				require.NoError(t, base.Symlink("untouched", target))
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			require.NoError(t, backupFS.RemoveAll(target))
			mustNotExist(t, base, target)
			fileMustContainText(t, base, untouched, "untouched contents")
			require.Zero(t, counting.calls[untouched], "do not resolve or inspect a final symlink's target")
			require.LessOrEqual(t, counting.calls[target], test.maxLstat, "reuse metadata already observed by RemoveAll")
			for _, filePath := range files {
				require.Equal(t, 1, counting.calls[filePath], "reuse file metadata from traversal")
			}
			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

type removeAllFailureFS struct {
	FS
	statPath   string
	removePath string
}

func (fsys removeAllFailureFS) Lstat(name string) (fs.FileInfo, error) {
	if name == fsys.statPath {
		return nil, os.ErrPermission
	}
	return fsys.FS.Lstat(name)
}

func (fsys removeAllFailureFS) Remove(name string) error {
	if name == fsys.removePath {
		return os.ErrPermission
	}
	return fsys.FS.Remove(name)
}

func TestBackupFS_RemoveAllPreservesErrorsAndRollback(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"initial lookup", "traversal", "removal"} {
		t.Run(stage, func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			target := testutils.AbsFilePath(t, "/test/removeall/target")
			first, second := filepath.Join(target, "first"), filepath.Join(target, "second")
			createFile(t, base, first, "first contents")
			createFile(t, base, second, "second contents")
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			failures := removeAllFailureFS{FS: backupFS.base}
			switch stage {
			case "initial lookup":
				failures.statPath = target
			case "traversal":
				failures.statPath = second
			case "removal":
				failures.removePath = second
			}
			backupFS.base = failures
			err := backupFS.RemoveAll(target)
			require.ErrorIs(t, err, os.ErrPermission)
			var pathError *os.PathError
			require.ErrorAs(t, err, &pathError)
			require.Equal(t, "remove_all", pathError.Op)
			require.Equal(t, target, pathError.Path)
			if stage == "initial lookup" {
				mustEqualFSState(t, baseFSState, base, "/")
			} else {
				mustNotExist(t, base, first)
				fileMustContainText(t, base, second, "second contents")
			}
			backupFS.base = failures.FS
			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_Rename(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)
	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		oldDirName   = "/test/rename"
		newDirName   = "/test/rename2"
		newerDirName = "/test/rename3"
	)

	err := base.MkdirAll(oldDirName, 0755)
	require.NoError(err)
	mustExist(t, base, oldDirName)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	err = backupFS.Rename(oldDirName, newDirName)
	require.NoError(err)

	mustNotExist(t, backupFS, oldDirName)
	mustExist(t, backupFS, newDirName)

	mustNotExist(t, base, oldDirName)
	mustExist(t, base, newDirName)

	mustNotExist(t, backup, newDirName)
	mustExist(t, backup, oldDirName)

	err = backupFS.Rename(newDirName, newerDirName)
	require.NoError(err)

	mustNotExist(t, backupFS, newDirName)
	mustExist(t, backupFS, newerDirName)

	mustExist(t, backup, oldDirName)
	mustNotExist(t, backup, newDirName)
	mustNotExist(t, backup, newerDirName)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_RenameOverwriteRollback(t *testing.T) {
	t.Parallel()

	for _, existingSource := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing source", false: "temporary source"}[existingSource], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			source := testutils.AbsFilePath(t, "/test/rename/source")
			target := testutils.AbsFilePath(t, "/test/rename/target")
			createFile(t, base, target, "original target")
			if existingSource {
				createFile(t, base, source, "original source")
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")

			for index, content := range []string{"first replacement", "second replacement"} {
				if existingSource && index == 0 {
					content = "original source"
				} else {
					createFile(t, backupFS, source, content)
				}
				require.NoError(t, backupFS.Rename(source, target))
				mustNotExist(t, base, source)
				fileMustContainText(t, base, target, content)
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_RenameAvoidsDuplicateLstat(t *testing.T) {
	t.Parallel()

	for _, existingTarget := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing target", false: "missing target"}[existingTarget], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			source := testutils.AbsFilePath(t, "/test/rename/source")
			target := testutils.AbsFilePath(t, "/test/rename/target")
			createFile(t, base, source, "source contents")
			if existingTarget {
				createFile(t, base, target, "target contents")
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			require.NoError(t, backupFS.Rename(source, target))
			require.Equal(t, 1, counting.calls[source], "reuse source metadata from path resolution")
			require.Equal(t, 1, counting.calls[target], "reuse destination metadata from path resolution")
			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_BackupRequiredResolvesKnownSymlinkInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}
	_, base, _, backupFS := NewTestBackupFS(t)
	target := testutils.AbsFilePath(t, "/test/target")
	link := testutils.AbsFilePath(t, "/test/link")
	createFile(t, base, target, "target contents")
	createSymlink(t, base, target, link)
	linkInfo, err := base.Lstat(link)
	require.NoError(t, err)
	info, required, err := backupFS.backupRequired(target, linkInfo)
	require.NoError(t, err)
	require.True(t, required)
	require.True(t, info.Mode().IsRegular(), "snapshot metadata must describe the resolved file, not its symlink")
}

func TestBackupFS_Rollback(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// delete directory & files that did exist before
	removeAll(t, backupFS, fileDir)

	// removed files must not exist
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, backup, fileDirRoot)
	mustExist(t, backup, fileDir)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test01.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test02.txt"), fileContent)

	// create files that did not exist before
	createFile(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	mustNotExist(t, backup, filepath.Join(fileDir2, "/test05_new.txt"))

	// create subdir of deleted directory which did not exist before
	mkdirAll(t, backupFS, "/test/001/subdir_new", 0755)
	createFile(t, backupFS, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	mustNotExist(t, backup, "/test/001/subdir_new")
	mustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err := backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	mustExist(t, backupFS, fileDir)
	mustExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	// also restored in the underlying filesystem
	mustExist(t, base, fileDir)
	mustExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	// newly created files must have been deleted upon rollback
	mustNotExist(t, base, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"))

	// new files should have been deleted
	mustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	mustNotExist(t, backupFS, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	mustNotExist(t, base, "/test/001/subdir_new")
	mustNotExist(t, backupFS, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	mustExist(t, base, fileDir)
	mustExist(t, backupFS, fileDir)

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_RollbackWithForcedBackup(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	// delete directory & files that did exist before
	removeAll(t, backupFS, fileDir)

	// force  backup of a deleted directoty
	//  which existed before
	err := backupFS.ForceBackup(fileDir)
	require.NoError(err)

	// removed files must not exist
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, backup, fileDirRoot)
	mustNotExist(t, backup, fileDir)
	mustNotExist(t, backup, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backup, filepath.Join(fileDir, "/test02.txt"))

	// create files that did not exist before
	createFile(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)
	createFile(t, backupFS, filepath.Join(fileDir2, "/test06_new.txt"), fileContentNew)

	mustNotExist(t, backup, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, backup, filepath.Join(fileDir2, "/test06_new.txt"))

	err = backupFS.ForceBackup(filepath.Join(fileDir2, "/test05_new.txt"))
	require.NoError(err)

	fileMustContainText(t, backup, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)

	mkdirAll(t, backupFS, "/test/001/subdir_new", 0755)
	createFile(t, backupFS, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	mustNotExist(t, backup, "/test/001/subdir_new")
	mustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, base, filepath.Join(fileDir2, "/test05_new.txt"))
	mustExist(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, base, filepath.Join(fileDir2, "/test06_new.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir2, "/test06_new.txt"))

	mustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	mustNotExist(t, backupFS, "/test/001/subdir_new/test06_new.txt")

	mustNotExist(t, base, "/test/001/subdir_new")
	mustNotExist(t, backupFS, "/test/001/subdir_new")

	// we forced the deletion of the fileDir to be backed up
	// this means the the folder and its contents do not exist anymore
	mustNotExist(t, base, fileDir)
	mustNotExist(t, backupFS, fileDir)
}

func TestBackupFS_ForceBackupReusesResolvedMetadata(t *testing.T) {
	t.Parallel()

	for _, existingFile := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing file", false: "newly created file"}[existingFile], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/force/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			previousSnapshot := ""
			if existingFile {
				previousSnapshot = "original contents"
				createFile(t, base, filePath, previousSnapshot)
			}
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting
			acceptedTime := time.Unix(1_600_000_000, 0)

			for _, contents := range []string{"first accepted state", "second accepted state"} {
				createFile(t, backupFS, filePath, contents)
				require.NoError(t, base.Chmod(filePath, 0640))
				require.NoError(t, base.Chtimes(filePath, acceptedTime, acceptedTime))
				if previousSnapshot == "" {
					mustNotExist(t, backup, filePath)
				} else {
					fileMustContainText(t, backup, filePath, previousSnapshot)
				}
				counting.calls[filePath] = 0
				require.NoError(t, backupFS.ForceBackup(filePath))
				require.Equal(t, 1, counting.calls[filePath], "reuse live metadata after removing the old backup")
				fileMustContainText(t, backup, filePath, contents)
				info, err := backup.Lstat(filePath)
				require.NoError(t, err)
				modeMustBeEqual(t, 0640, info.Mode())
				require.True(t, acceptedTime.Equal(info.ModTime()), "refresh snapshot metadata as well as contents")
				previousSnapshot = contents
			}

			acceptedFSState := createFSState(t, base, "/")
			createFile(t, backupFS, filePath, "later changes")
			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, acceptedFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
			info, err := base.Lstat(filePath)
			require.NoError(t, err)
			require.True(t, acceptedTime.Equal(info.ModTime()), "rollback must restore the refreshed modification time")
		})
	}
}

func TestBackupFS_ForceBackupDeletedFileReusesResolvedMetadata(t *testing.T) {
	t.Parallel()
	_, base, backup, backupFS := NewTestBackupFS(t)
	filePath := testutils.AbsFilePath(t, "/test/force/file")
	createFile(t, base, filePath, "original contents")
	backupFSState := createFSState(t, backup, "/")
	counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
	backupFS.base = counting
	removeFile(t, backupFS, filePath)
	fileMustContainText(t, backup, filePath, "original contents")
	deletedFSState := createFSState(t, base, "/")
	counting.calls[filePath] = 0
	require.NoError(t, backupFS.ForceBackup(filePath))
	require.Equal(t, 1, counting.calls[filePath], "reuse the resolved absence of the live file")
	mustNotExist(t, backup, filePath)
	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, deletedFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_JSON(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot    = "/test"
		fileDir        = "/test/001"
		fileDir2       = "/test/0/2"
		fileContent    = "test_content"
		fileContentNew = "test_content_new"
	)

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir, "/test02.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test03.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test04.txt"), fileContent)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// delete directory & files that did exist before
	removeAll(t, backupFS, fileDir)

	// removed files must not exist
	mustNotExist(t, base, fileDir)
	mustNotExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	mustNotExist(t, backupFS, fileDir)
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test01.txt"))
	mustNotExist(t, backupFS, filepath.Join(fileDir, "/test02.txt"))

	mustExist(t, backup, fileDirRoot)
	mustExist(t, backup, fileDir)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test01.txt"), fileContent)
	fileMustContainText(t, backup, filepath.Join(fileDir, "/test02.txt"), fileContent)

	// create files that did not exist before
	createFile(t, backupFS, filepath.Join(fileDir2, "/test05_new.txt"), fileContentNew)

	// must not exist becaus eit's a new file that did not exist in the base fs before.
	mustNotExist(t, backup, filepath.Join(fileDir2, "/test05_new.txt"))

	// create subdir of deleted directory which did not exist before
	mkdirAll(t, backupFS, "/test/001/subdir_new", 0755)
	createFile(t, backupFS, "/test/001/subdir_new/test06_new.txt", "fileContentNew")

	// must also not exist becaus ethese are new files
	mustNotExist(t, backup, "/test/001/subdir_new")
	mustNotExist(t, backup, "/test/001/subdir_new/test06_new.txt")

	// JSON
	// after unmarshalling we should have the exact same behavior as without the marshaling/unmarshaling
	data, err := json.Marshal(backupFS)
	require.NoError(err)

	backupFSNew := NewBackupFS(base, backup)
	err = json.Unmarshal(data, &backupFSNew)
	require.NoError(err)

	// JSON
	oldMap := backupFS.baseInfos
	newMap := backupFSNew.baseInfos

	for path, info := range oldMap {
		newInfo := newMap[path]

		if info == nil {
			require.Nil(newInfo)
			continue
		}

		require.Equal(info.IsDir(), newInfo.IsDir())
		require.Equal(info.Name(), newInfo.Name())
		require.Equal(info.Size(), newInfo.Size())
		require.Equal(info.ModTime().UnixNano(), newInfo.ModTime().UnixNano())
		require.Equal(info.Mode(), newInfo.Mode())

		if runtime.GOOS != "windows" {
			require.Greater(toUID(info), -1)
			require.Greater(toGID(info), -1)

			require.Greater(toUID(newInfo), -1)
			require.Greater(toGID(newInfo), -1)
		}
	}

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// previously deleted files must have been restored
	mustExist(t, backupFSNew, fileDir)
	mustExist(t, backupFSNew, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, backupFSNew, filepath.Join(fileDir, "/test02.txt"))

	// also restored in the underlying filesystem
	mustExist(t, base, fileDir)
	mustExist(t, base, filepath.Join(fileDir, "/test01.txt"))
	mustExist(t, base, filepath.Join(fileDir, "/test02.txt"))

	// newly created files must have been deleted upon rollback
	mustNotExist(t, base, filepath.Join(fileDir2, "/test05_new.txt"))
	mustNotExist(t, backupFSNew, filepath.Join(fileDir2, "/test05_new.txt"))

	// new files should have been deleted
	mustNotExist(t, base, "/test/001/subdir_new/test06_new.txt")
	mustNotExist(t, backupFSNew, "/test/001/subdir_new/test06_new.txt")

	// new directories as well
	mustNotExist(t, base, "/test/001/subdir_new")
	mustNotExist(t, backupFSNew, "/test/001/subdir_new")

	// but old directories that did exist before should still exist
	mustExist(t, base, "/test/001")
	mustExist(t, backupFSNew, "/test/001")

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_Symlink(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		err     error
		require = require.New(t)
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = testutils.AbsFilePath(t, "/test")
		fileDir     = testutils.AbsFilePath(t, "/test/001")
		fileDir2    = testutils.AbsFilePath(t, "/test/0/2")
		fileContent = "test_content"
	)

	// base filesystem structure and files before modifying

	mkdirAll(t, base, fileDir, 0755)
	mkdirAll(t, base, fileDir2, 0755)

	createFile(t, base, filepath.Join(fileDir, "/test01.txt"), fileContent)
	createFile(t, base, filepath.Join(fileDir2, "/test02.txt"), fileContent)

	createSymlink(t, base, filepath.Join(fileDir, "/test01.txt"), filepath.Join(fileDirRoot, "/file_symlink"))
	createSymlink(t, base, fileDir, filepath.Join(fileDirRoot, "/directory_symlink"))

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// modify through BackupFS layer

	// the old symlink must have been backed up after this call

	removeFile(t, backupFS, filepath.Join(fileDirRoot, "/file_symlink"))
	removeFile(t, backupFS, filepath.Join(fileDirRoot, "/directory_symlink"))

	// potential problem case:
	// Symlink creation fails midway due to another file, directory or symlink already existing.
	// due to the writing character of the symlink method we do create a backup
	// but fail to create a new symlink thus the backed up file and the old symlink are indeed the exact same
	// not exactly a problem but may cause unnecessary backed up data
	createSymlink(t, backupFS, filepath.Join(fileDir2, "/test02.txt"), filepath.Join(fileDirRoot, "/file_symlink"))

	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir2, "/test02.txt"))

	symlinkMustExistWithTragetPath(t, backup, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir, "/test01.txt"))

	// create folder symlinks
	createSymlink(t, backupFS, fileDir2, filepath.Join(fileDirRoot, "/directory_symlink"))
	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir2)
	symlinkMustExistWithTragetPath(t, backup, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir)

	createSymlink(t, backupFS, filepath.Join(fileDir2, "/does_not_exist"), "/to_be_removed_symlink")

	err = backupFS.Rollback()
	require.NoError(err)

	// assert both base symlinks point to their respective previous paths
	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir, "/test01.txt"))
	symlinkMustExistWithTragetPath(t, backupFS, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir)

	symlinkMustExistWithTragetPath(t, base, filepath.Join(fileDirRoot, "/file_symlink"), filepath.Join(fileDir, "/test01.txt"))
	symlinkMustExistWithTragetPath(t, base, filepath.Join(fileDirRoot, "/directory_symlink"), fileDir)

	// never existed before, was created and then rolled back
	mustNotLExist(t, backupFS, "/to_be_removed_symlink")

	mustNotLExist(t, backup, filepath.Join(fileDirRoot, "/file_symlink"))
	mustNotLExist(t, backup, filepath.Join(fileDirRoot, "/directory_symlink"))

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")

}

func TestBackupFS_SymlinkReusesResolvedMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}
	t.Parallel()

	for _, test := range []struct {
		name          string
		existingFile  bool
		existingLink  bool
		missingSource bool
	}{
		{name: "new relative link"},
		{name: "dangling relative link", missingSource: true},
		{name: "existing file destination", existingFile: true},
		{name: "existing link destination", existingLink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			parent := testutils.AbsFilePath(t, "/test/symlink")
			mkdirAll(t, base, parent, 0755)
			source, destination := filepath.Join(parent, "source"), filepath.Join(parent, "link")
			if !test.missingSource {
				createFile(t, base, source, "source contents")
			}
			if test.existingFile {
				createFile(t, base, destination, "original destination")
			} else if test.existingLink {
				require.NoError(t, base.Symlink("source", destination))
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			err := backupFS.Symlink("source", destination)
			if test.existingFile || test.existingLink {
				require.ErrorIs(t, err, os.ErrExist)
				mustEqualFSState(t, baseFSState, base, "/")
			} else {
				require.NoError(t, err)
				linked, err := base.Readlink(destination)
				require.NoError(t, err)
				require.Equal(t, "source", linked, "preserve the caller's relative link target")
			}
			require.Equal(t, 1, counting.calls[destination], "reuse destination metadata from path resolution")
			if test.existingLink {
				require.Equal(t, 1, counting.calls[source], "read fresh metadata for the resolved target, not the link")
				info, err := backup.Lstat(source)
				require.NoError(t, err)
				require.True(t, info.Mode().IsRegular())
				fileMustContainText(t, backup, source, "source contents")
			} else {
				require.Zero(t, counting.calls[source], "do not resolve the caller's link target during creation")
			}
			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_Mkdir(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		fileDir     = "/test/001"
		fileDir2    = "/test/001/002"
	)

	err := mkdir(t, base, fileDirRoot, 0755)
	require.NoError(err)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	err = mkdir(t, backupFS, fileDir2, 0755)
	require.Error(err, "cannot create child directory without having created its parent")

	err = mkdir(t, backupFS, fileDir, 0755)
	require.NoError(err)

	err = mkdir(t, backupFS, fileDir2, 0755)
	require.NoError(err)

	removeAll(t, backupFS, fileDirRoot)

	// /test existed in the base filesystem and has been removed at the end -> upon removal we backup this directory.
	mustLExist(t, backup, fileDirRoot)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_MkdirReusesResolvedMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		existingDir   bool
		existingFile  bool
		missingParent bool
		wantErr       error
		maxLstat      int
	}{
		{name: "new directory", maxLstat: 1},
		{name: "existing directory", existingDir: true, wantErr: os.ErrExist, maxLstat: 2},
		{name: "existing file", existingFile: true, wantErr: os.ErrExist, maxLstat: 1},
		{name: "missing parent", missingParent: true, wantErr: os.ErrNotExist, maxLstat: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			parent := testutils.AbsFilePath(t, "/test/mkdir")
			mkdirAll(t, base, parent, 0755)
			target := filepath.Join(parent, "target")
			if test.existingDir {
				mkdirAll(t, base, target, 0700)
				createFile(t, base, filepath.Join(target, "original"), "original contents")
			} else if test.existingFile {
				createFile(t, base, target, "original contents")
			} else if test.missingParent {
				target = filepath.Join(parent, "missing", "target")
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			err := backupFS.Mkdir(target, 0755)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				mustEqualFSState(t, baseFSState, base, "/")
			} else {
				require.NoError(t, err)
				info, err := base.Lstat(target)
				require.NoError(t, err)
				require.True(t, info.IsDir())
				modeMustBeEqual(t, 0755, info.Mode())
			}
			require.LessOrEqual(t, counting.calls[target], test.maxLstat, "avoid re-querying metadata established during path resolution")
			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_MkdirAll(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/Program Data"
		fileDir     = "/Program Data/001"
		fileDir2    = "/Program Data/001/002"
	)

	// already existing files before we touched the filesystem
	err := mkdir(t, base, fileDirRoot, 0755)
	require.NoError(err)

	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// at this point writing operations must happen on backupFS
	// and read operations should happen on any of the tree, base, backup or backupFS
	mkdirAll(t, backupFS, fileDir2, 0755)
	removeAll(t, backupFS, fileDir)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_MkdirAllReusesResolvedMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		existingDir      bool
		existingFile     bool
		missingAncestors bool
		maxLstat         int
	}{
		{name: "new directory", maxLstat: 1},
		{name: "existing directory", existingDir: true, maxLstat: 2},
		{name: "missing ancestors", missingAncestors: true, maxLstat: 0},
		{name: "existing file", existingFile: true, maxLstat: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			parent := testutils.AbsFilePath(t, "/test/mkdirall")
			mkdirAll(t, base, parent, 0755)
			target := filepath.Join(parent, "target")
			if test.existingDir {
				mkdirAll(t, base, target, 0700)
				createFile(t, base, filepath.Join(target, "original"), "original contents")
			} else if test.existingFile {
				createFile(t, base, target, "original contents")
			} else if test.missingAncestors {
				target = filepath.Join(parent, "first", "second", "target")
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for attempt := 0; attempt < 2; attempt++ {
				counting.calls[target] = 0
				err := backupFS.MkdirAll(target, 0755)
				if test.existingFile {
					require.Error(t, err)
					var pathError *os.PathError
					require.ErrorAs(t, err, &pathError)
					require.Equal(t, "mkdir_all", pathError.Op)
					require.Equal(t, target, pathError.Path)
					mustEqualFSState(t, baseFSState, base, "/")
				} else {
					require.NoError(t, err)
					info, err := base.Lstat(target)
					require.NoError(t, err)
					require.True(t, info.IsDir())
					if test.existingDir {
						modeMustBeEqual(t, 0700, info.Mode())
						fileMustContainText(t, base, filepath.Join(target, "original"), "original contents")
					} else {
						modeMustBeEqual(t, 0755, info.Mode())
					}
				}
				budget := test.maxLstat
				if attempt > 0 && budget == 0 {
					budget = 1
				}
				require.LessOrEqual(t, counting.calls[target], budget, "avoid re-querying metadata established during path resolution")
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_Chmod(t *testing.T) {
	t.Parallel()

	var (
		require = require.New(t)
	)

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// different number of file path separators
		// while still having the same number of characters in the filepath
		fileDirRoot = "/test"
		filePath    = filepath.Join(fileDirRoot, "/test_file_chmod.txt")
	)
	createFile(t, base, filePath, "chmod test file")

	// get initial permission bits
	initialFi, err := base.Lstat(filePath)
	require.NoError(err)
	initialMode := initialFi.Mode()

	// snapshot filesystem states
	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")

	// change mod
	expectedNewPerm := fs.FileMode(0644)
	chmod(t, backupFS, filePath, expectedNewPerm)

	// get backed up file permissions
	fi, err := backup.Lstat(filePath)
	require.NoError(err)

	// compare backed up permissions to initial permissions
	backedUpPerm := fi.Mode()
	modeMustBeEqual(t, initialMode, backedUpPerm)

	// ROLLBACK
	err = backupFS.Rollback()
	require.NoError(err)
	// ROLLBACK

	// compare initial state to state after rollback
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_ChmodAvoidsDuplicateLstat(t *testing.T) {
	t.Parallel()

	for _, existingFile := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing file", false: "missing file"}[existingFile], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/chmod/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			var initialMode fs.FileMode
			if existingFile {
				createFile(t, base, filePath, "original contents")
				chmod(t, base, filePath, 0600)
				info, err := base.Lstat(filePath)
				require.NoError(t, err)
				initialMode = info.Mode()
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for _, mode := range []fs.FileMode{0444, 0644} {
				counting.calls[filePath] = 0
				err := backupFS.Chmod(filePath, mode)
				if existingFile {
					require.NoError(t, err)
					info, err := base.Lstat(filePath)
					require.NoError(t, err)
					modeMustBeEqual(t, mode, info.Mode())
					original, err := backup.Lstat(filePath)
					require.NoError(t, err)
					modeMustBeEqual(t, initialMode, original.Mode())
					fileMustContainText(t, base, filePath, "original contents")
				} else {
					require.ErrorIs(t, err, os.ErrNotExist)
					mustNotExist(t, backup, filePath)
				}
				require.Equal(t, 1, counting.calls[filePath], "reuse file metadata from path resolution")
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
		})
	}
}

func TestBackupFS_ChownAvoidsDuplicateLstat(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Chown is unsupported and ignored on Windows")
	}
	t.Parallel()

	for _, existingFile := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing file", false: "missing file"}[existingFile], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/chown/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			ownerInfo, err := base.Lstat(filepath.Dir(filePath))
			require.NoError(t, err)
			uid, gid := toUID(ownerInfo), toGID(ownerInfo)
			if existingFile {
				createFile(t, base, filePath, "original contents")
				ownerInfo, err = base.Lstat(filePath)
				require.NoError(t, err)
				uid, gid = toUID(ownerInfo), toGID(ownerInfo)
			}
			require.GreaterOrEqual(t, uid, 0)
			require.GreaterOrEqual(t, gid, 0)
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for attempt := 0; attempt < 2; attempt++ {
				counting.calls[filePath] = 0
				err := backupFS.Chown(filePath, uid, gid)
				if existingFile {
					require.NoError(t, err)
					for _, filesystem := range []FS{base, backup} {
						info, err := filesystem.Lstat(filePath)
						require.NoError(t, err)
						require.Equal(t, uid, toUID(info))
						require.Equal(t, gid, toGID(info))
					}
					fileMustContainText(t, base, filePath, "original contents")
				} else {
					require.ErrorIs(t, err, os.ErrNotExist)
					mustNotExist(t, backup, filePath)
				}
				require.Equal(t, 1, counting.calls[filePath], "reuse file metadata from path resolution")
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
			if existingFile {
				info, err := base.Lstat(filePath)
				require.NoError(t, err)
				require.Equal(t, uid, toUID(info))
				require.Equal(t, gid, toGID(info))
			}
		})
	}
}

type chownDeniedFS struct{ FS }

func (chownDeniedFS) Chown(string, int, int) error { return os.ErrPermission }

func TestBackupFS_ChownPreservesPermissionError(t *testing.T) {
	t.Parallel()
	_, base, backup, backupFS := NewTestBackupFS(t)
	filePath := testutils.AbsFilePath(t, "/test/chown/file")
	createFile(t, base, filePath, "original contents")
	baseFSState := createFSState(t, base, "/")
	backupFSState := createFSState(t, backup, "/")
	backupFS.base = chownDeniedFS{FS: backupFS.base}
	err := backupFS.Chown(filePath, -1, -1)
	require.ErrorIs(t, err, os.ErrPermission)
	var pathError *os.PathError
	require.ErrorAs(t, err, &pathError)
	require.Equal(t, "chown", pathError.Op)
	require.Equal(t, filePath, pathError.Path)
	fileMustContainText(t, base, filePath, "original contents")
	fileMustContainText(t, backup, filePath, "original contents")
	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseFSState, base, "/")
	mustEqualFSState(t, backupFSState, backup, "/")
}

func TestBackupFS_ChtimesAvoidsDuplicateLstat(t *testing.T) {
	t.Parallel()

	for _, existingFile := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing file", false: "missing file"}[existingFile], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/chtimes/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			originalTime := time.Unix(1_600_000_000, 0)
			if existingFile {
				createFile(t, base, filePath, "original contents")
				require.NoError(t, base.Chtimes(filePath, originalTime, originalTime))
				info, err := base.Lstat(filePath)
				require.NoError(t, err)
				originalTime = info.ModTime()
			}
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for _, changedTime := range []time.Time{originalTime.Add(time.Hour), originalTime.Add(2 * time.Hour)} {
				counting.calls[filePath] = 0
				err := backupFS.Chtimes(filePath, changedTime, changedTime)
				if existingFile {
					require.NoError(t, err)
					info, err := base.Lstat(filePath)
					require.NoError(t, err)
					require.True(t, changedTime.Equal(info.ModTime()), "apply requested modification time")
					original, err := backup.Lstat(filePath)
					require.NoError(t, err)
					require.True(t, originalTime.Equal(original.ModTime()), "retain the original modification-time snapshot")
					fileMustContainText(t, base, filePath, "original contents")
				} else {
					require.ErrorIs(t, err, os.ErrNotExist)
					mustNotExist(t, backup, filePath)
				}
				require.Equal(t, 1, counting.calls[filePath], "reuse file metadata from path resolution")
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
			if existingFile {
				info, err := base.Lstat(filePath)
				require.NoError(t, err)
				require.True(t, originalTime.Equal(info.ModTime()), "restore original modification time after rollback")
			}
		})
	}
}

func TestBackupFS_LchownReusesResolvedMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Lchown is unsupported on Windows")
	}
	t.Parallel()

	for _, existingFile := range []bool{true, false} {
		t.Run(map[bool]string{true: "regular file", false: "missing file"}[existingFile], func(t *testing.T) {
			_, base, backup, backupFS := NewTestBackupFS(t)
			filePath := testutils.AbsFilePath(t, "/test/lchown/file")
			mkdirAll(t, base, filepath.Dir(filePath), 0755)
			ownerInfo, err := base.Lstat(filepath.Dir(filePath))
			require.NoError(t, err)
			uid, gid := toUID(ownerInfo), toGID(ownerInfo)
			if existingFile {
				createFile(t, base, filePath, "original contents")
				ownerInfo, err = base.Lstat(filePath)
				require.NoError(t, err)
				uid, gid = toUID(ownerInfo), toGID(ownerInfo)
			}
			require.GreaterOrEqual(t, uid, 0)
			require.GreaterOrEqual(t, gid, 0)
			baseFSState := createFSState(t, base, "/")
			backupFSState := createFSState(t, backup, "/")
			counting := &lstatCountingFS{FS: backupFS.base, calls: make(map[string]int)}
			backupFS.base = counting

			for attempt := 0; attempt < 2; attempt++ {
				counting.calls[filePath] = 0
				err := backupFS.Lchown(filePath, uid, gid)
				if existingFile {
					require.NoError(t, err)
					for _, filesystem := range []FS{base, backup} {
						info, err := filesystem.Lstat(filePath)
						require.NoError(t, err)
						require.Equal(t, uid, toUID(info))
						require.Equal(t, gid, toGID(info))
					}
					fileMustContainText(t, base, filePath, "original contents")
				} else {
					require.ErrorIs(t, err, os.ErrNotExist)
					mustNotExist(t, backup, filePath)
				}
				budget := 1
				if attempt > 0 {
					budget = 0
				}
				require.Equal(t, budget, counting.calls[filePath], "query the original file once, then reuse its snapshot")
			}

			require.NoError(t, backupFS.Rollback())
			mustEqualFSState(t, baseFSState, base, "/")
			mustEqualFSState(t, backupFSState, backup, "/")
			if existingFile {
				info, err := base.Lstat(filePath)
				require.NoError(t, err)
				require.Equal(t, uid, toUID(info))
				require.Equal(t, gid, toGID(info))
			}
		})
	}
}

func TestTime(t *testing.T) {
	require := require.New(t)

	t1 := time.Now()
	nanoBefore := t1.UnixNano()

	t2 := time.Unix(nanoBefore/1000000000, nanoBefore%1000000000)
	require.Equal(t1.UnixNano(), t2.UnixNano())
}

// this helper function is needed in order to test on the local filesystem
// and not in memory
func NewTempDirPrefixFS(rootDir string) *PrefixFS {
	var osFS = NewOSFS()

	tempDir, err := TempDir(osFS, rootDir, TimesStamp())
	if err != nil {
		panic(err)
	}

	pfs, err := NewPrefixFS(osFS, tempDir)
	if err != nil {
		panic(err)
	}
	return pfs
}

func NewTestBackupFS(t *testing.T) (root, base, backup FS, backupFS *BackupFS) {
	rootPath := CallerPathTmp()
	root = NewTempDirPrefixFS(rootPath)
	require := require.New(t)

	pwd, err := os.Getwd()
	require.NoError(err)

	volume := filepath.VolumeName(pwd)
	volumeName := strings.TrimRight(volume, ":")

	basePath := filepath.FromSlash(volume + "/base")

	err = root.MkdirAll(basePath, 0700)
	require.NoError(err)

	absBackupDirPath := filepath.Join(basePath, volumeName, "/backup")
	err = root.MkdirAll(absBackupDirPath, 0700)
	require.NoError(err)

	absBackupFilePath := filepath.Join(basePath, volumeName, "backup.file")
	f, err := root.Create(absBackupFilePath)
	require.NoError(err)
	defer f.Close()

	_, err = io.WriteString(f, "backup file")
	require.NoError(err)

	base, err = NewPrefixFS(root, basePath)
	require.NoError(err)

	err = base.MkdirAll(volume+"/", 0700)
	require.NoError(err)

	backup, err = NewPrefixFS(base, volume+"/backup")
	require.NoError(err)

	err = backup.MkdirAll(volume+"/", 0700)
	require.NoError(err)

	// hide backup locations in base filesystem
	base, err = NewHiddenFS(base, volume+"/backup", volume+"/backup.file")
	require.NoError(err)

	backupFS = NewBackupFS(
		base,
		backup,
	)

	return root, base, backup, backupFS
}

func TestBackupFS_CreateFileInSymlinkDir(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkSubDir       = path.Join(symlinkDir, "/systemd/system")
		symlinkFilePath     = path.Join(symlinkSubDir, "test.txt")

		updatedFileContent = "updated_content"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir)
	createFile(t, base, originalFilePath, originalFileContent)

	baseFsState := createFSState(t, base, "/")
	backupFsState := createFSState(t, backup, "/")

	// try creating the directory tree ober a symlinked folder
	createFile(t, backupFS, symlinkFilePath, updatedFileContent)

	err := backupFS.Rollback()
	require.NoError(t, err)

	mustEqualFSState(t, baseFsState, base, "/")
	mustEqualFSState(t, backupFsState, backup, "/")
}

func TestBackupFS_MkdirInSymlinkDir(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir)
	createFile(t, base, originalFilePath, originalFileContent)

	baseFsState := createFSState(t, base, "/")
	backupFsState := createFSState(t, backup, "/")

	// try creating the directory tree ober a symlinked folder
	mkdir(t, backupFS, filepath.Join(symlinkDir, "test_dir"), 0755)

	err := backupFS.Rollback()
	require.NoError(t, err)

	mustEqualFSState(t, baseFsState, base, "/")
	mustEqualFSState(t, backupFsState, backup, "/")
}

func TestBackupFS_RemoveDirInSymlinkDir(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		originalLinkedDir   = "/usr/lib"
		originalSubDir      = path.Join(originalLinkedDir, "/systemd/system")
		originalFilePath    = path.Join(originalSubDir, "test.txt")
		originalFileContent = "test_content"
		symlinkDir          = "/lib"
		symlinkSubDir       = "/lib/systemd"
	)

	// prepare existing files
	mkdirAll(t, base, originalSubDir, 0755)
	createSymlink(t, base, originalLinkedDir, symlinkDir)
	createFile(t, base, originalFilePath, originalFileContent)

	baseFsState := createFSState(t, base, "/")
	backupFsState := createFSState(t, backup, "/")

	// try creating the directory tree ober a symlinked folder
	removeAll(t, backupFS, symlinkSubDir)

	err := backupFS.Rollback()
	require.NoError(t, err)

	mustEqualFSState(t, baseFsState, base, "/")
	mustEqualFSState(t, backupFsState, backup, "/")
}

func PathTmp(funcName string) string {
	// Use the OS temp dir, never a project-local ./tmp. On macOS the project dir
	// lives under ~/Desktop, which is indexed by Spotlight and backed up by Time
	// Machine; writing hundreds of thousands of scratch dirs there triggers an
	// mds/fseventsd indexing storm that can freeze the host. The OS temp dir is
	// not indexed and is purged automatically. Cross-volume temp paths (e.g. the
	// Windows CI runner's C: temp vs a D: checkout) are handled by the HiddenFS
	// containment checks treating other-volume paths as not-contained.
	return filepath.Join(os.TempDir(), "backupfs-test", funcName)
}

func CallerPathTmp(up ...int) string {
	caller := 1
	if len(up) > 0 {
		caller += up[0]
	}
	return PathTmp(testutils.CallerFuncName(caller))
}

func FuncPathTmp(up ...int) string {
	caller := 1
	if len(up) > 0 {
		caller += up[0]
	}
	return PathTmp(testutils.FuncName(caller + 1))
}

// TestBackupFS_RemoveFileSymlink tests the behavior of BackupFS when removing a directory
// that contains symlinks pointing to files and directories outside the removed directory.
//
// This test verifies several critical aspects of BackupFS symlink handling:
// 1. Symlinks are properly backed up when their containing directory is removed
// 2. Target files/directories remain intact when symlinks pointing to them are removed
// 3. Rollback functionality correctly restores all symlinks and directory structure
// 4. Filesystem state is completely preserved across remove/rollback operations
//
// Test Setup:
// Creates a directory structure with two separate directory trees:
//   - /dir1/ - Contains symlinks that will be removed
//     ├── link_to_file -> /dir2/dir3/file.txt (symlink to file)
//     └── link_to_dir -> /dir2/dir3/ (symlink to directory)
//   - /dir2/dir3/ - Contains target resources
//     └── file.txt - Target file with test content
//
// Test Flow:
// 1. Setup: Create directory structure, target file, and symlinks
// 2. Verify: Confirm all symlinks exist and point to correct targets
// 3. Capture: Record initial filesystem state for both base and backup filesystems
// 4. Remove: Delete /dir1/ (containing symlinks) using BackupFS.RemoveAll()
// 5. Verify: Ensure symlinks are removed but targets remain intact
// 6. Verify: Confirm symlinks and directory are properly backed up
// 7. Rollback: Restore filesystem to initial state using BackupFS.Rollback()
// 8. Verify: Confirm filesystem state matches exactly with initial capture
// 9. Verify: Ensure all symlinks, targets, and directory structure are restored
func TestBackupFS_RemoveFileSymlink(t *testing.T) {
	t.Parallel()

	// Initialize BackupFS test environment with base, backup, and backupFS instances
	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// Directory structure layout:
		// /dir1/link_to_file -> /dir2/dir3/file.txt (file symlink)
		// /dir1/link_to_dir -> /dir2/dir3 (directory symlink)
		dir1        = testutils.AbsFilePath(t, "/dir1")      // Directory containing symlinks (to be removed)
		dir3        = testutils.AbsFilePath(t, "/dir2/dir3") // Target directory for symlinks
		targetFile  = filepath.Join(dir3, "file.txt")        // Target file for file symlink
		linkToFile  = filepath.Join(dir1, "link_to_file")    // Symlink pointing to targetFile
		linkToDir   = filepath.Join(dir1, "link_to_dir")     // Symlink pointing to dir3
		fileContent = "test content for symlink target"      // Content for target file
	)

	// === SETUP PHASE ===
	// Create the required directory structure in the base filesystem
	mkdirAll(t, base, dir1, 0755) // Create directory that will contain symlinks
	mkdirAll(t, base, dir3, 0755) // Create target directory for symlinks

	// Create the target file that one of the symlinks will point to
	createFile(t, base, targetFile, fileContent)
	fileMustContainText(t, base, targetFile, fileContent)

	// Create symlinks in dir1 pointing to targets in dir2/dir3
	createSymlink(t, base, targetFile, linkToFile) // File symlink: dir1/link_to_file -> dir2/dir3/file.txt
	createSymlink(t, base, dir3, linkToDir)        // Directory symlink: dir1/link_to_dir -> dir2/dir3

	// === VERIFICATION PHASE ===
	// Verify that symlinks were created successfully and point to correct targets
	symlinkMustExist(t, base, linkToFile)
	symlinkMustExist(t, base, linkToDir)
	symlinkMustExistWithTragetPath(t, base, linkToFile, targetFile)
	symlinkMustExistWithTragetPath(t, base, linkToDir, dir3)

	// === STATE CAPTURE PHASE ===
	// Capture initial filesystem states before any modifications
	// This serves as the baseline for rollback verification
	initialFSState := createFSState(t, base, "/")
	initialBackupFSState := createFSState(t, backup, "/")

	// === REMOVAL PHASE ===
	// Remove dir1 (containing both symlinks) using BackupFS.RemoveAll
	// This should backup the directory and its symlinks before removal
	removeAll(t, backupFS, dir1)

	// === POST-REMOVAL VERIFICATION PHASE ===
	// Verify that dir1 and its symlinks are completely removed from base filesystem
	mustNotExist(t, backupFS, dir1)    // Directory should not exist via BackupFS
	mustNotExist(t, base, dir1)        // Directory should not exist in base filesystem
	mustNotLExist(t, base, linkToFile) // File symlink should not exist
	mustNotLExist(t, base, linkToDir)  // Directory symlink should not exist

	// Verify that symlink targets remain intact (removal of symlinks shouldn't affect targets)
	mustExist(t, base, targetFile)                        // Target file should still exist
	mustExist(t, base, dir3)                              // Target directory should still exist
	fileMustContainText(t, base, targetFile, fileContent) // Target file content should be unchanged

	// Verify that removed directory and symlinks were properly backed up
	mustExist(t, backup, dir1)              // Directory should exist in backup
	mustLExist(t, backup, linkToFile)       // File symlink should be backed up
	mustLExist(t, backup, linkToDir)        // Directory symlink should be backed up
	symlinkMustExist(t, backup, linkToFile) // Backed up file symlink should be valid
	symlinkMustExist(t, backup, linkToDir)  // Backed up directory symlink should be valid

	// === ROLLBACK PHASE ===
	// Test rollback functionality to restore the filesystem to initial state
	err := backupFS.Rollback()
	require.NoError(t, err)

	// === POST-ROLLBACK VERIFICATION PHASE ===
	// Capture filesystem states after rollback for comparison
	finalFSState := createFSState(t, base, "/")
	finalBackupFSState := createFSState(t, backup, "/")

	// Verify that rollback completely restored the initial filesystem state
	mustEqualFSState(t, initialFSState, base, "/")
	mustEqualFSState(t, initialBackupFSState, backup, "/")

	// Perform explicit state comparison to ensure perfect restoration
	require.Equal(t, initialFSState, finalFSState, "Filesystem state before removal and after rollback should be identical")
	require.Equal(t, initialBackupFSState, finalBackupFSState, "Backup filesystem state before removal and after rollback should be identical")

	// Verify that all components are restored and functional after rollback
	mustExist(t, base, dir1)                                        // Directory containing symlinks should be restored
	symlinkMustExist(t, base, linkToFile)                           // File symlink should be restored
	symlinkMustExist(t, base, linkToDir)                            // Directory symlink should be restored
	symlinkMustExistWithTragetPath(t, base, linkToFile, targetFile) // File symlink should point to correct target
	symlinkMustExistWithTragetPath(t, base, linkToDir, dir3)        // Directory symlink should point to correct target
	mustExist(t, base, targetFile)                                  // Target file should still exist
	mustExist(t, base, dir3)                                        // Target directory should still exist
	fileMustContainText(t, base, targetFile, fileContent)           // Target file content should remain unchanged
}

// TestBackupFS_RemoveIndividualSymlinks tests the behavior of BackupFS when removing
// individual symlinks using Remove() (not RemoveAll()).
//
// This test verifies several critical aspects of BackupFS individual symlink removal:
// 1. Individual symlinks are properly backed up when removed using Remove()
// 2. Target files/directories remain intact when symlinks pointing to them are removed
// 3. Rollback functionality correctly restores individual symlinks
// 4. Filesystem state is completely preserved across remove/rollback operations
//
// Test Setup:
// Creates individual symlinks pointing to target resources:
//   - /dir2/dir3/file.txt - Target file with test content
//   - /link_to_file -> /dir2/dir3/file.txt (file symlink to be removed)
//   - /link_to_dir -> /dir2/dir3/ (directory symlink to be removed)
//
// Test Flow:
// 1. Setup: Create target file and individual symlinks
// 2. Verify: Confirm all symlinks exist and point to correct targets
// 3. Capture: Record initial filesystem state for both base and backup filesystems
// 4. Remove: Delete individual symlinks using BackupFS.Remove()
// 5. Verify: Ensure symlinks are removed but targets remain intact
// 6. Verify: Confirm symlinks are properly backed up
// 7. Rollback: Restore filesystem to initial state using BackupFS.Rollback()
// 8. Verify: Confirm filesystem state matches exactly with initial capture
// 9. Verify: Ensure all symlinks and targets are restored
func TestBackupFS_RemoveIndividualSymlinks(t *testing.T) {
	t.Parallel()

	// Initialize BackupFS test environment with base, backup, and backupFS instances
	_, base, backup, backupFS := NewTestBackupFS(t)

	var (
		// Individual symlink structure layout:
		// /link_to_file -> /dir2/dir3/file.txt (file symlink)
		// /link_to_dir -> /dir2/dir3 (directory symlink)
		dir3        = testutils.AbsFilePath(t, "/dir2/dir3")       // Target directory for symlinks
		targetFile  = filepath.Join(dir3, "file.txt")              // Target file for file symlink
		linkToFile  = testutils.AbsFilePath(t, "/link_to_file")    // File symlink (to be removed individually)
		linkToDir   = testutils.AbsFilePath(t, "/link_to_dir")     // Directory symlink (to be removed individually)
		fileContent = "test content for individual symlink target" // Content for target file
	)

	// === SETUP PHASE ===
	// Create the target directory and file in the base filesystem
	mkdirAll(t, base, dir3, 0755) // Create target directory for symlinks

	// Create the target file that one of the symlinks will point to
	createFile(t, base, targetFile, fileContent)
	fileMustContainText(t, base, targetFile, fileContent)

	// Create individual symlinks pointing to targets
	createSymlink(t, base, targetFile, linkToFile) // File symlink: /link_to_file -> /dir2/dir3/file.txt
	createSymlink(t, base, dir3, linkToDir)        // Directory symlink: /link_to_dir -> /dir2/dir3

	// === VERIFICATION PHASE ===
	// Verify that symlinks were created successfully and point to correct targets
	symlinkMustExist(t, base, linkToFile)
	symlinkMustExist(t, base, linkToDir)
	symlinkMustExistWithTragetPath(t, base, linkToFile, targetFile)
	symlinkMustExistWithTragetPath(t, base, linkToDir, dir3)

	// === STATE CAPTURE PHASE ===
	// Capture initial filesystem states before any modifications
	// This serves as the baseline for rollback verification
	initialFSState := createFSState(t, base, "/")
	initialBackupFSState := createFSState(t, backup, "/")

	// === REMOVAL PHASE ===
	// Remove individual symlinks using BackupFS.Remove (not RemoveAll)
	// This should backup each symlink before removal
	err := backupFS.Remove(linkToFile)
	require.NoError(t, err)

	err = backupFS.Remove(linkToDir)
	require.NoError(t, err)

	// === POST-REMOVAL VERIFICATION PHASE ===
	// Verify that individual symlinks are completely removed from base filesystem
	mustNotLExist(t, backupFS, linkToFile) // File symlink should not exist via BackupFS
	mustNotLExist(t, base, linkToFile)     // File symlink should not exist in base filesystem
	mustNotLExist(t, backupFS, linkToDir)  // Directory symlink should not exist via BackupFS
	mustNotLExist(t, base, linkToDir)      // Directory symlink should not exist in base filesystem

	// Verify that symlink targets remain intact (removal of symlinks shouldn't affect targets)
	mustExist(t, base, targetFile)                        // Target file should still exist
	mustExist(t, base, dir3)                              // Target directory should still exist
	fileMustContainText(t, base, targetFile, fileContent) // Target file content should be unchanged

	// Verify that removed symlinks were properly backed up
	mustLExist(t, backup, linkToFile)       // File symlink should be backed up
	mustLExist(t, backup, linkToDir)        // Directory symlink should be backed up
	symlinkMustExist(t, backup, linkToFile) // Backed up file symlink should be valid
	symlinkMustExist(t, backup, linkToDir)  // Backed up directory symlink should be valid

	// === ROLLBACK PHASE ===
	// Test rollback functionality to restore the filesystem to initial state
	err = backupFS.Rollback()
	require.NoError(t, err)

	// === POST-ROLLBACK VERIFICATION PHASE ===
	// Capture filesystem states after rollback for comparison
	finalFSState := createFSState(t, base, "/")
	finalBackupFSState := createFSState(t, backup, "/")

	// Verify that rollback completely restored the initial filesystem state
	mustEqualFSState(t, initialFSState, base, "/")
	mustEqualFSState(t, initialBackupFSState, backup, "/")

	// Perform explicit state comparison to ensure perfect restoration
	require.Equal(t, initialFSState, finalFSState, "Filesystem state before removal and after rollback should be identical")
	require.Equal(t, initialBackupFSState, finalBackupFSState, "Backup filesystem state before removal and after rollback should be identical")

	// Verify that all components are restored and functional after rollback
	symlinkMustExist(t, base, linkToFile)                           // File symlink should be restored
	symlinkMustExist(t, base, linkToDir)                            // Directory symlink should be restored
	symlinkMustExistWithTragetPath(t, base, linkToFile, targetFile) // File symlink should point to correct target
	symlinkMustExistWithTragetPath(t, base, linkToDir, dir3)        // Directory symlink should point to correct target
	mustExist(t, base, targetFile)                                  // Target file should still exist
	mustExist(t, base, dir3)                                        // Target directory should still exist
	fileMustContainText(t, base, targetFile, fileContent)           // Target file content should remain unchanged
}

func TimesStamp() string {
	return time.Now().Format("2006-01-02_15-04-05.000")
}
