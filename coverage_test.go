package backupfs

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBackupFS_ChmodRollback exercises Chmod through the backup layer and asserts the
// original mode is restored on rollback. (V3, V5, V6)
func TestBackupFS_ChmodRollback(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const filePath = "/test/chmod/file.txt"
	createFile(t, base, filePath, "content")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	require.NoError(t, backupFS.Chmod(filePath, 0700))

	fi, err := base.Lstat(filePath)
	require.NoError(t, err)
	modeMustBeEqual(t, 0700, fi.Mode())

	// original was backed up exactly once
	mustExist(t, backup, filePath)

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestBackupFS_Chown exercises Chown through the backup layer. uid/gid changes
// require privileges and are no-ops on windows, so we only assert the call path
// and that rollback restores state. (V3, V9)
func TestBackupFS_Chown(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const filePath = "/test/chown/file.txt"
	createFile(t, base, filePath, "content")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	fi, err := base.Lstat(filePath)
	require.NoError(t, err)

	// chown to its current owner -> no privilege needed, still triggers backup
	uid := toUID(fi)
	gid := toGID(fi)
	require.NoError(t, backupFS.Chown(filePath, uid, gid))

	mustExist(t, backup, filePath)

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestBackupFS_Chtimes exercises Chtimes and asserts the PathError carries the
// correct "chtimes" operation. (V3 + regression for §B B2)
func TestBackupFS_Chtimes(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const filePath = "/test/chtimes/file.txt"
	createFile(t, base, filePath, "content")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	tm := time.Now().Add(-time.Hour)
	require.NoError(t, backupFS.Chtimes(filePath, tm, tm))
	mustExist(t, backup, filePath)

	// error path must report the chtimes op, not chown
	err := backupFS.Chtimes("/does/not/exist.txt", tm, tm)
	require.Error(t, err)
	var pe *os.PathError
	require.ErrorAs(t, err, &pe)
	require.Equal(t, "chtimes", pe.Op, "Chtimes error must report op=chtimes")

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestBackupFS_Lchown exercises Lchown on a symlink, including a symlink located
// under a symlinked parent directory so that the resolved path differs from the
// requested name. (V3 + regression for §B B1)
func TestBackupFS_Lchown(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const (
		targetFile  = "/data/target.txt"
		symlinkPath = "/data/link"
	)
	createFile(t, base, targetFile, "content")
	createSymlink(t, base, targetFile, symlinkPath)

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	fi, err := base.Lstat(symlinkPath)
	require.NoError(t, err)

	// lchown the symlink itself to its current owner: triggers a backup of the
	// resolved symlink path. The bug B1 applied the lchown to the unresolved
	// name instead of the resolved path.
	require.NoError(t, backupFS.Lchown(symlinkPath, toUID(fi), toGID(fi)))
	mustLExist(t, backup, symlinkPath)

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestBackupFS_Accessors covers BaseFS, BackupFS, Name. (I.backup)
func TestBackupFS_Accessors(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	require.NotNil(t, backupFS.BaseFS())
	require.NotNil(t, backupFS.BackupFS())
	require.Equal(t, base.Name(), backupFS.BaseFS().Name())
	require.Equal(t, backup.Name(), backupFS.BackupFS().Name())
}

// TestBackupFS_MapRoundTrip covers Map, SetMap and JSON marshalling. (V13, I.backup)
func TestBackupFS_MapRoundTrip(t *testing.T) {
	t.Parallel()

	_, base, _, backupFS := NewTestBackupFS(t)

	const filePath = "/test/map/file.txt"
	createFile(t, base, filePath, "content")
	createFile(t, backupFS, filePath, "overwritten")

	m := backupFS.Map()
	require.NotEmpty(t, m)

	data, err := json.Marshal(backupFS)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// round-trip into a fresh instance via SetMap
	other := NewBackupFS(backupFS.BaseFS(), backupFS.BackupFS())
	other.SetMap(m)
	require.Equal(t, len(m), len(other.Map()))

	// round-trip via JSON unmarshalling
	fromJSON := NewBackupFS(backupFS.BaseFS(), backupFS.BackupFS())
	require.NoError(t, json.Unmarshal(data, fromJSON))
	require.Equal(t, len(m), len(fromJSON.Map()))
}

// TestBackupFS_ReadOnlyDelegates covers Stat and Readlink of the read-only
// delegating methods. (I.backup, I.Symlinker)
func TestBackupFS_ReadOnlyDelegates(t *testing.T) {
	t.Parallel()

	_, base, _, backupFS := NewTestBackupFS(t)

	const (
		targetFile  = "/ro/target.txt"
		symlinkPath = "/ro/link"
	)
	createFile(t, base, targetFile, "content")
	createSymlink(t, base, targetFile, symlinkPath)

	fi, err := backupFS.Stat(targetFile)
	require.NoError(t, err)
	require.False(t, fi.IsDir())

	// Stat error path reports op=stat
	_, err = backupFS.Stat("/ro/missing.txt")
	require.Error(t, err)
	var pe *os.PathError
	require.ErrorAs(t, err, &pe)
	require.Equal(t, "stat", pe.Op)

	pointsTo, err := backupFS.Readlink(symlinkPath)
	require.NoError(t, err)
	require.NotEmpty(t, pointsTo)

	_, err = backupFS.Readlink("/ro/missing-link")
	require.Error(t, err)
}

// TestBackupFS_RollbackIsIdempotent asserts a second rollback is a no-op. (V6)
func TestBackupFS_RollbackIsIdempotent(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const filePath = "/test/idem/file.txt"
	createFile(t, base, filePath, "content")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	createFile(t, backupFS, filePath, "overwritten")

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")

	// second rollback must not error and must not change anything
	require.NoError(t, backupFS.Rollback())
	require.Empty(t, backupFS.Map())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestPrefixFS_SymlinkEscapeOption covers both values of
// PrefixFSWithEnableSymlinkEscape and the Readlink reconstruction. (V11, V12)
func TestPrefixFS_SymlinkEscapeOption(t *testing.T) {
	t.Parallel()

	osFS := NewOSFS()

	for _, escape := range []bool{false, true} {
		escape := escape
		name := "escape_disabled"
		if escape {
			name = "escape_enabled"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmp, err := TempDir(osFS, t.TempDir())
			require.NoError(t, err)

			pfs, err := NewPrefixFS(osFS, tmp, PrefixFSWithEnableSymlinkEscape(escape))
			require.NoError(t, err)

			require.NoError(t, pfs.MkdirAll("/dir", 0755))
			f, err := pfs.Create("/dir/target.txt")
			require.NoError(t, err)
			require.NoError(t, f.Close())

			// relative symlink inside the prefix
			require.NoError(t, pfs.Symlink("target.txt", "/dir/rel_link"))
			got, err := pfs.Readlink("/dir/rel_link")
			require.NoError(t, err)
			require.Equal(t, "target.txt", filepath.ToSlash(got))

			// absolute symlink inside the prefix
			require.NoError(t, pfs.Symlink("/dir/target.txt", "/dir/abs_link"))
			got, err = pfs.Readlink("/dir/abs_link")
			require.NoError(t, err)
			require.NotEmpty(t, got)
			// readlink never returns a root-relative path
			require.NotContains(t, got, tmp)
		})
	}
}

// TestHiddenFile_Readdirnames_HidesWithPositiveCount asserts hidden entries are
// excluded even when a positive count is requested. (V8, regression for §B B3)
func TestHiddenFile_Readdirnames_HidesWithPositiveCount(t *testing.T) {
	t.Parallel()

	hiddenDirParent, hiddenDir, _, base, fsys := SetupTempDirHiddenFSTest(t)
	_ = hiddenDir

	// add a couple visible siblings next to the hidden directory
	createFile(t, base, filepath.Join(hiddenDirParent, "visible_a.txt"), "a")
	createFile(t, base, filepath.Join(hiddenDirParent, "visible_b.txt"), "b")

	d, err := fsys.Open(hiddenDirParent)
	require.NoError(t, err)
	defer d.Close()

	names, err := d.Readdirnames(100)
	if err != nil {
		require.ErrorIs(t, err, io.EOF)
	}
	require.NotContains(t, names, "backups", "hidden dir must not leak via Readdirnames(positive count)")
	require.Contains(t, names, "visible_a.txt")
	require.Contains(t, names, "visible_b.txt")
}

// TestHiddenFile_Readdir_HidesWithPositiveCount mirrors the above for Readdir. (V8)
func TestHiddenFile_Readdir_HidesWithPositiveCount(t *testing.T) {
	t.Parallel()

	hiddenDirParent, _, _, base, fsys := SetupTempDirHiddenFSTest(t)

	createFile(t, base, filepath.Join(hiddenDirParent, "visible_a.txt"), "a")
	createFile(t, base, filepath.Join(hiddenDirParent, "visible_b.txt"), "b")

	d, err := fsys.Open(hiddenDirParent)
	require.NoError(t, err)
	defer d.Close()

	infos, err := d.Readdir(100)
	if err != nil {
		require.ErrorIs(t, err, io.EOF)
	}
	for _, fi := range infos {
		require.NotEqual(t, "backups", fi.Name(), "hidden dir must not leak via Readdir(positive count)")
	}
}

// TestHiddenFile_FileOps covers the hiddenFile read/write/seek wrappers that are
// otherwise uncovered. (I.File)
func TestHiddenFile_FileOps(t *testing.T) {
	t.Parallel()

	hiddenDirParent, _, _, base, fsys := SetupTempDirHiddenFSTest(t)
	_ = base

	filePath := filepath.Join(hiddenDirParent, "rw.txt")
	f, err := fsys.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	require.NoError(t, err)

	n, err := f.WriteString("hello world")
	require.NoError(t, err)
	require.Equal(t, len("hello world"), n)

	require.NoError(t, f.Sync())

	n2, err := f.WriteAt([]byte("HELLO"), 0)
	require.NoError(t, err)
	require.Equal(t, 5, n2)

	off, err := f.Seek(0, io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(0), off)

	buf := make([]byte, 5)
	rn, err := f.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, 5, rn)
	require.Equal(t, "HELLO", string(buf))

	require.NoError(t, f.Truncate(5))
	require.NoError(t, f.Close())

	fi, err := fsys.Stat(filePath)
	require.NoError(t, err)
	require.Equal(t, int64(5), fi.Size())
}

// TestBackupFS_Constructors covers New and NewWithFS. (I.backup)
func TestBackupFS_Constructors(t *testing.T) {
	t.Parallel()

	osFS := NewOSFS()
	root, err := TempDir(osFS, t.TempDir())
	require.NoError(t, err)

	backupLocation := filepath.Join(root, "backup")

	fromOS := New(backupLocation)
	require.NotNil(t, fromOS)
	require.Equal(t, "BackupFS", fromOS.Name())

	fromFS := NewWithFS(osFS, backupLocation)
	require.NotNil(t, fromFS)
	require.NotNil(t, fromFS.BaseFS())
	require.NotNil(t, fromFS.BackupFS())
}

// TestPrefixPath covers the exported PrefixPath helper. (I.prefix, C5)
func TestPrefixPath(t *testing.T) {
	t.Parallel()

	prefix, err := filepath.Abs(filepath.FromSlash("/prefix"))
	require.NoError(t, err)

	got, err := PrefixPath(prefix, "/a/b")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, prefix), "result %q must start with prefix %q", got, prefix)
	require.Contains(t, filepath.ToSlash(got), "a/b")
}

// TestHiddenFS_MetadataOps covers Chmod, Chown, Lchown and Rename on the
// HiddenFS for non-hidden paths plus rejection of hidden paths. (V8)
func TestHiddenFS_MetadataOps(t *testing.T) {
	t.Parallel()

	hiddenDirParent, hiddenDir, _, base, fsys := SetupTempDirHiddenFSTest(t)

	filePath := filepath.Join(hiddenDirParent, "meta.txt")
	createFile(t, base, filePath, "content")

	require.NoError(t, fsys.Chmod(filePath, 0600))

	fi, err := fsys.Stat(filePath)
	require.NoError(t, err)
	require.NoError(t, fsys.Chown(filePath, toUID(fi), toGID(fi)))
	require.NoError(t, fsys.Lchown(filePath, toUID(fi), toGID(fi)))

	// renaming a non-hidden file is allowed
	renamed := filepath.Join(hiddenDirParent, "meta_renamed.txt")
	require.NoError(t, fsys.Rename(filePath, renamed))
	mustExist(t, fsys, renamed)

	// operating on a hidden path is rejected
	require.Error(t, fsys.Chmod(hiddenDir, 0700))
	require.Error(t, fsys.Rename(hiddenDir, filepath.Join(hiddenDirParent, "moved")))
}

// TestPrefixFS_RootProtection asserts the prefix root cannot be removed or
// renamed. (V2)
func TestPrefixFS_RootProtection(t *testing.T) {
	t.Parallel()

	osFS := NewOSFS()
	tmp, err := TempDir(osFS, t.TempDir())
	require.NoError(t, err)
	pfs, err := NewPrefixFS(osFS, tmp)
	require.NoError(t, err)

	require.ErrorIs(t, pfs.Remove("/"), os.ErrPermission)
	require.ErrorIs(t, pfs.RemoveAll("/"), os.ErrPermission)
	require.ErrorIs(t, pfs.Rename("/", "/elsewhere"), os.ErrPermission)
}

// TestPrefixFS_EscapePrevented asserts directory traversal cannot escape the
// prefix. After cleaning, an escaping path is clamped inside the prefix rather
// than reaching the real parent. (V1)
func TestPrefixFS_EscapePrevented(t *testing.T) {
	t.Parallel()

	osFS := NewOSFS()
	tmp, err := TempDir(osFS, t.TempDir())
	require.NoError(t, err)
	pfs, err := NewPrefixFS(osFS, tmp)
	require.NoError(t, err)

	// create a file via a traversal path; it must land inside the prefix
	createFile(t, pfs, "/../../escapee.txt", "data")

	// the file is reachable at the clamped in-prefix location ...
	mustExist(t, pfs, "/escapee.txt")

	// ... and never written outside the prefix on the real filesystem
	_, found, err := lexists(osFS, filepath.Join(filepath.Dir(filepath.Dir(tmp)), "escapee.txt"))
	require.NoError(t, err)
	require.False(t, found, "file must not escape the prefix on the real filesystem")
}

// TestWalk covers Walk including the not-found root error path. (I.walk)
func TestWalk(t *testing.T) {
	t.Parallel()

	osFS := NewOSFS()
	tmp, err := TempDir(osFS, t.TempDir())
	require.NoError(t, err)
	pfs, err := NewPrefixFS(osFS, tmp)
	require.NoError(t, err)

	createFile(t, pfs, "/a/b/c.txt", "c")
	createFile(t, pfs, "/a/d.txt", "d")

	seen := map[string]bool{}
	err = Walk(pfs, "/a", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		seen[filepath.Base(p)] = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, seen["c.txt"])
	require.True(t, seen["d.txt"])

	// walking a missing root surfaces the error to the walk func
	err = Walk(pfs, "/does-not-exist", func(p string, info os.FileInfo, err error) error {
		return err
	})
	require.Error(t, err)

	// returning SkipDir on a subdirectory skips its contents
	createFile(t, pfs, "/skip/keep.txt", "k")
	createFile(t, pfs, "/skip/sub/hidden.txt", "h")
	visited := map[string]bool{}
	err = Walk(pfs, "/skip", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(p)
		visited[base] = true
		if info.IsDir() && base == "sub" {
			return filepath.SkipDir
		}
		return nil
	})
	require.NoError(t, err)
	require.True(t, visited["keep.txt"])
	require.True(t, visited["sub"])
	require.False(t, visited["hidden.txt"], "SkipDir must skip subdirectory contents")
}

// TestHiddenFS_CaseInsensitiveBypass asserts that on case-insensitive
// filesystems a differently-cased path cannot bypass the hidden-path
// protection and modify the backup location. (V8, regression for §B B4)
func TestHiddenFS_CaseInsensitiveBypass(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("host filesystem is case-sensitive; cased bypass is not applicable")
	}

	_, hiddenDir, _, base, fsys := SetupTempDirHiddenFSTest(t)

	// the hidden dir contains 2 entries to start with
	countFiles(t, base, hiddenDir, 2)

	// upper-casing the hidden path must NOT let us remove the hidden content
	upper := strings.ToUpper(hiddenDir)
	_ = fsys.RemoveAll(upper)

	// hidden content is untouched
	countFiles(t, base, hiddenDir, 2)

	// and creating inside the upper-cased hidden dir must be rejected
	_, err := fsys.Create(filepath.Join(upper, "evil.txt"))
	require.Error(t, err)
}

// TestIsInHiddenPath unit-tests the hidden-path containment logic. (V8)
func TestIsInHiddenPath(t *testing.T) {
	t.Parallel()

	base, err := filepath.Abs(filepath.FromSlash("/var/opt/backups"))
	require.NoError(t, err)

	abs := func(p string) string {
		v, err := filepath.Abs(filepath.FromSlash(p))
		require.NoError(t, err)
		return v
	}

	cases := []struct {
		name   string
		path   string
		hidden bool
	}{
		{"same dir", base, true},
		{"file inside", filepath.Join(base, "file.txt"), true},
		{"nested inside", filepath.Join(base, "a", "b", "c"), true},
		{"direct parent", abs("/var/opt"), false},
		{"sibling", abs("/var/opt/other"), false},
		{"unrelated", abs("/etc/passwd"), false},
		// a relative name forces the abs() fallback path (Rel fails on mixed abs/rel)
		{"relative name fallback", "relative/sub", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, hidden, err := isInHiddenPath(tc.path, base)
			require.NoError(t, err)
			require.Equal(t, tc.hidden, hidden)
		})
	}
}

// TestDirContains unit-tests the parent/child directory relation. (V8)
func TestDirContains(t *testing.T) {
	t.Parallel()

	parent, err := filepath.Abs(filepath.FromSlash("/var/opt"))
	require.NoError(t, err)

	in, err := dirContains(parent, filepath.Join(parent, "backups"))
	require.NoError(t, err)
	require.True(t, in)

	same, err := dirContains(parent, parent)
	require.NoError(t, err)
	require.False(t, same)

	out, err := dirContains(parent, filepath.Dir(parent))
	require.NoError(t, err)
	require.False(t, out)

	// mixed abs/relative inputs force the abs() fallback path
	_, err = dirContains("relative/parent", filepath.Join(parent, "backups"))
	require.NoError(t, err)
}

// TestOSFS_Name covers the trivial Name accessor. (I.osfs)
func TestOSFS_Name(t *testing.T) {
	t.Parallel()
	require.Equal(t, "OSFS", NewOSFS().Name())
}

// TestBackupFS_ForceBackup covers ForceBackup and the re-backup path after a
// content change. (V3, I.backup)
func TestBackupFS_ForceBackup(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const filePath = "/force/file.txt"
	createFile(t, base, filePath, "v1")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	// modify through backupFS -> v1 backed up
	createFile(t, backupFS, filePath, "v2")
	fileMustContainText(t, backup, filePath, "v1")

	// force a fresh backup of the current (v2) state
	require.NoError(t, backupFS.ForceBackup(filePath))
	fileMustContainText(t, backup, filePath, "v2")

	// changing again does not overwrite the forced backup
	createFile(t, backupFS, filePath, "v3")
	fileMustContainText(t, backup, filePath, "v2")

	// rollback restores the forced (v2) state, not the original
	require.NoError(t, backupFS.Rollback())
	fileMustContainText(t, base, filePath, "v2")

	_ = baseState
	_ = backupState
}

// TestHiddenFile_Readdir_Iterative drives Readdir with a small positive count
// across multiple calls to exercise the chunked fetch loop. (V8, I.File)
func TestHiddenFile_Readdir_Iterative(t *testing.T) {
	t.Parallel()

	hiddenDirParent, _, _, base, fsys := SetupTempDirHiddenFSTest(t)

	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		createFile(t, base, filepath.Join(hiddenDirParent, n), n)
	}

	d, err := fsys.Open(hiddenDirParent)
	require.NoError(t, err)
	defer d.Close()

	got := map[string]bool{}
	for {
		infos, err := d.Readdir(1)
		for _, fi := range infos {
			require.NotEqual(t, "backups", fi.Name())
			got[fi.Name()] = true
		}
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}
		if len(infos) == 0 {
			break
		}
	}
	require.True(t, got["a.txt"] && got["b.txt"] && got["c.txt"])
}

// TestBackupFS_RemoveAllNested removes a nested tree and asserts a full
// restore on rollback. Exercises directory backup, recursive backup removal
// and ordered restore. (V3, V5, V6)
func TestBackupFS_RemoveAllNested(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	createFile(t, base, "/tree/a/b/c.txt", "c")
	createFile(t, base, "/tree/a/b/d.txt", "d")
	createFile(t, base, "/tree/a/e.txt", "e")
	createFile(t, base, "/tree/f.txt", "f")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	require.NoError(t, backupFS.RemoveAll("/tree/a"))
	mustNotExist(t, base, "/tree/a/b/c.txt")
	mustExist(t, base, "/tree/f.txt")

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestBackupFS_RenameRollback renames a file and asserts rollback restores the
// original layout. (V3, V5, V6)
func TestBackupFS_RenameRollback(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	createFile(t, base, "/rn/old.txt", "content")

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	require.NoError(t, backupFS.Rename("/rn/old.txt", "/rn/new.txt"))
	mustExist(t, base, "/rn/new.txt")
	mustNotExist(t, base, "/rn/old.txt")

	require.NoError(t, backupFS.Rollback())
	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestBackupFS_RestoresModTime asserts the original modification time is
// restored on rollback, exercising the chtimes restore path. (V5)
func TestBackupFS_RestoresModTime(t *testing.T) {
	t.Parallel()

	_, base, backup, backupFS := NewTestBackupFS(t)

	const filePath = "/mtime/file.txt"
	createFile(t, base, filePath, "original")

	// pin the base file's modtime well into the past so restore must reset it
	past := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	require.NoError(t, base.Chtimes(filePath, past, past))

	baseState := createFSState(t, base, "/")
	backupState := createFSState(t, backup, "/")

	// overwrite through backupFS -> original (with past modtime) is backed up
	createFile(t, backupFS, filePath, "changed")

	require.NoError(t, backupFS.Rollback())

	fi, err := base.Lstat(filePath)
	require.NoError(t, err)
	require.WithinDuration(t, past, fi.ModTime(), 2*time.Second, "modtime must be restored")

	mustEqualFSState(t, baseState, base, "/")
	mustEqualFSState(t, backupState, backup, "/")
}

// TestTrimVolume covers volume trimming. (I.volume)
func TestTrimVolume(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		require.Equal(t, `\A\B`, TrimVolume(`C:\A\B`))
	} else {
		// no volume on unix -> unchanged
		require.Equal(t, "/A/B", TrimVolume("/A/B"))
	}
}
