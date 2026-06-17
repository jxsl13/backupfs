# SPEC — backupfs

Caveman spec. Distilled from code @ `1286343`; updated on branch `coverage-bugfixes-cicd` (B1–B5 fixes, CI, coverage). `?` = unconfirmed, user verify.

## §G — goal

Go lib. Stacked filesystem layers give backup-on-write + rollback. App modify file/dir/symlink via wrapper → original backed up first. `Rollback()` restore start state, delete backup.

## §C — constraints

- C1 Go `1.21`+ (go.mod). Test matrix `stable` + `oldstable`.
- C2 Cross-OS: linux, darwin, windows. Same behavior all three. Platform split files (`*_unix.go`, `*_windows.go`).
- C3 Deps minimal: only `testify` (test). No runtime deps.
- C4 Windows symlink need Developer Mode or SeCreateSymbolicLink policy.
- C5 Windows: no `:` in backup paths → volume `C:` mapped to `\C\` (normalizeVolumePath).
- C6 All FS impls satisfy `FS` interface (filesystem.go). Compile-time assert `var _ FS = (*T)(nil)`.

## §I — interfaces (public surface)

- I.FS — `FS` interface: Create, Mkdir, MkdirAll, Open, OpenFile, Remove, RemoveAll, Rename, Stat, Name, Chmod, Chown, Chtimes + `Symlinker`. (filesystem.go)
- I.Symlinker — Lstat, Symlink, Readlink, Lchown.
- I.File — `File` interface: fs.File + Reader/Writer/Seeker/ReaderAt/WriterAt + Readdir, Readdirnames, Sync, Truncate, WriteString.
- I.osfs — `NewOSFS() OSFS`. Real OS passthrough.
- I.prefix — `NewPrefixFS(fsys, prefixPath, opts...)`, opt `PrefixFSWithEnableSymlinkEscape(bool)`, helper `PrefixPath(prefix, name)`.
- I.hidden — `NewHiddenFS(base, hiddenPaths...)`. Hide backup loc, block recursion.
- I.backup — `New(loc, opts...)`, `NewWithFS(baseFS, loc, opts...)`, `NewBackupFS(base, backup, opts...)`. Methods: `Rollback()`, `ForceBackup(name)`, `BaseFS()`, `BackupFS()`, `Map()`, `SetMap()`, Marshal/UnmarshalJSON.
- I.walk — `Walk(fsys, root, walkFn)`.
- I.temp — `TempDir(fsys, dir, prefix...)`.
- I.sort — `ByMostFilePathSeparators`, `ByLeastFilePathSeparators`, `LessFilePathSeparators(a,b)`.
- I.volume — `TrimVolume(filePath)`.
- I.errs — `ErrRollbackFailed` (sentinel, joined on rollback fail).

## §V — invariants

- V1 PrefixFS: no escape prefix. Any path resolved abs+joined under prefix; if not HasPrefix(prefix) → `syscall.EPERM`. (prefixfs.go:110) Confirmed by FuzzPrefixFS.
- V2 PrefixFS: cannot Remove/RemoveAll/Rename the prefix root itself → `EPERM`. (prefixfs.go:201,222,242)
- V3 BackupFS: file backed up at most once. First modify backs up original state; later modifies skip (alreadySeen). (backupfs.go:1108 backupRequired)
- V4 BackupFS: file absent in base at modify time recorded as `nil` info; Rollback removes it (not restore). (backupfs.go:672,1120)
- V5 BackupFS: Rollback restores order = remove-base → restore dirs → files → symlinks; backup deletion order = symlink → file → dir (dir last so empty). (backupfs.go:711-754)
- V6 BackupFS: Rollback resets baseInfos to empty after run. (backupfs.go:764) Repeated Rollback = no-op.
- V7 BackupFS: Rollback failure wraps `ErrRollbackFailed` via errors.Join; continues best-effort, reports all errs. (backupfs.go:650)
- V8 HiddenFS: hidden paths invisible + unmodifiable from base; ops on hidden path error, listings exclude them. Prevents backup recursion. Confirmed by `FuzzHiddenFS_Create`, `FuzzHiddenFS_RemoveAll`.
- V9 Cross-OS Chown/Chtimes: errors that are "not implemented on this OS" ignorable (windows `EWINDOWS`); uid/gid = -1 on windows. (fs_utils_windows.go)
- V10 Sort: ByMost = descending path-separator count, ByLeast = ascending. Drives nesting order in rollback. (sort.go) Confirmed by FuzzSort*.
- V11 ? Symlink escape: default EnableSymlinkEscape=false → abs symlink targets rewritten to stay inside prefix; relative links resolved + kept relative; escape attempt errors. (prefixfs.go:338) `?` confirm exact policy.
- V12 ? Readlink never returns root-relative (`\C\..`) paths — abs or relative only; volume reconstructed on windows. (prefixfs.go:391,432)
- V13 BackupFS JSON: Map round-trips via Marshal/UnmarshalJSON; baseInfos serializable incl uid/gid sys (unix). (backupfs_json*.go) `?` confirm windows nil-sys round-trip.
- V14 BackupFS errors: PathError.Op equals the actual operation name (Chtimes reports "chtimes", not "chown"). (backupfs.go Chtimes) ← B2
- V15 BackupFS Lchown operates on the symlink itself: final path component NOT resolved through the link, and backup is taken of that same link (realParentPath, not realPath). (backupfs.go Lchown) ← B1
- V16 hiddenFile Readdir/Readdirnames exclude hidden entries for ANY count value (positive and non-positive); hiding uses the full joined path, never the bare basename. (hiddenfs_file.go) ← B3
- V17 Hidden-path protection is case-insensitive on case-insensitive filesystems (Windows, macOS): a differently-cased path cannot bypass hiding to read/modify/remove the backup location. Folding only widens hiding (safe direction). (hiddenfs.go foldPath) ← B4. Confirmed by FuzzHiddenFS_Create seed `/vAr/`.
- V18 HiddenFS containment: path on different volume than hidden dir → not hidden (`filepath.Rel` cross-volume failure ⇒ not-contained, ⊥ error). (hiddenfs.go isInHiddenPath, dirContains) ← B5

## §T — tasks

id|status|task|cites
T1|~|raise coverage to ≥80% total (64.7%→79.5% single-OS; ≥80% enforced on MERGED multi-OS via codecov.yml — Windows-only volume branches uncovered on darwin/linux)|V1-V18
T2|x|test BackupFS Chown/Chtimes/Lchown|V9,V14,V15,I.backup
T3|x|test BackupFS New/NewWithFS/BaseFS/BackupFS/SetMap/Map/JSON|V13,I.backup
T4|x|test hiddenfs_file.go Readdir/Readdirnames/Sync/Truncate/ReadAt/Seek/WriteAt|I.File,V8,V16
T5|x|test hiddenfs isInHiddenPath/dirContains edge cases incl rel-path fallback|V8
T6|x|test backupfs_ready_only.go Stat/Readlink|I.backup
T7|x|test fs_utils restore/copy paths via nested RemoveAll + modtime rollback|V5,V9
T8|x|test prefixfs symlink escape policy both option values|V11,V12
T9|~|Rollback repeated no-op done (V6); explicit ErrRollbackFailed injection still TODO|V6,V7
T10|x|fix bugs found → B1,B2,B3,B4,B5 fixed + recorded; V14-V18 added|§B
T11|x|CI: golangci-lint + gofmt job (.golangci.yml)|C1
T12|x|CI: build matrix GOOS=linux,darwin,windows × amd64,arm64|C2
T13|x|CI: gate coverage ≥80% on merged report (codecov.yml project status)|T1
T14|x|CI: scheduled+dispatch fuzz run, all 5 targets × 3 OS (fuzz.yaml)|V1,V8,V10,V17
T15|x|CI: release pipeline — tag push → cross-OS verify + gh release notes (release.yaml)|C1,C2
T16|x|CI: 3-OS × stable/oldstable test matrix kept; -race + per-flag codecov upload|C2
T17|~|Windows symlink: relies on GH windows-latest runner (symlinks enabled); existing+new symlink tests run there. Explicit Developer-Mode skip NOT added|C4,V11
T18|x|cross-volume HiddenFS containment (other-volume path ⇒ not-contained); covered by `TestHiddenFS_RelCantMakeRelative` on windows CI (C: temp vs D: checkout)|V18

## §B — bugs

id|date|cause|fix
B1|2026-06-17|BackupFS.Lchown backed up realPath (symlink RESOLVED to target) but called base.Lchown(name) on the UNRESOLVED path — backup path ≠ op path, and it chowned the link target instead of the link|use realParentPath (final component unresolved) for both backup and op → V15
B2|2026-06-17|BackupFS.Chtimes wrapped its error as PathError{Op:"chown"} (copy-paste) → misleading op name|set Op:"chtimes" → V14
B3|2026-06-17|hiddenFile.Readdirnames positive-count branch called isHidden(name) with the bare basename instead of the joined full path → hidden entries leaked when a positive count was requested|join hf.filePath before isHidden → V16
B4|2026-06-17|HiddenFS containment (isInHiddenPath/dirContains) compared paths case-SENSITIVELY; on case-insensitive FS (macOS/Windows) a differently-cased path (e.g. `/vAr/` vs `/var/`) bypassed protection and could remove/modify the hidden backup location. Found by FuzzHiddenFS_Create, pre-existing on master|foldPath case-folds operands on windows/darwin → V17
B5|2026-06-17|HiddenFS containment propagated `filepath.Rel` cross-volume error (windows hidden dir on C: vs query path on D:) instead of treating other-volume path as not-contained → ops errored `is_hidden ...: Rel: can't make d:\ relative to c:\...`. Surfaced by moving test temp to os.TempDir|Rel failure after abs fallback ⇒ return not-contained, nil → V18
