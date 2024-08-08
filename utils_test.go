package backupfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func fileMustContainText(t *testing.T, fsys FS, path, content string) {
	path = filepath.Clean(path)

	require := require.New(t)
	f, err := fsys.Open(path)
	require.NoError(err)
	defer f.Close()
	b, err := io.ReadAll(f)
	require.NoError(err)

	require.Equal(string(b), content)
}

func symlinkMustExist(t *testing.T, fsys FS, symlinkPath string) (path string) {
	symlinkPath = filepath.Clean(symlinkPath)

	require := require.New(t)

	fi, err := fsys.Lstat(symlinkPath)
	require.NotErrorIs(fs.ErrNotExist, err, "target symlink does not exist but is expected to exist: ", symlinkPath)
	require.NoError(err)

	hasSymlinkFlag := fi.Mode()&os.ModeType&os.ModeSymlink != 0
	require.Truef(hasSymlinkFlag, "target symlink does not have the symlink flag: %s", symlinkPath)

	actualPointsTo, err := fsys.Readlink(symlinkPath)
	require.NoError(err)

	require.True(actualPointsTo != "", "symlink target path is empty")
	return actualPointsTo
}

func symlinkMustExistWithTragetPath(t *testing.T, fsys FS, symlinkPath, expectedPointsTo string) {
	expectedPointsTo = filepath.Clean(expectedPointsTo)
	actualPointsTo := symlinkMustExist(t, fsys, symlinkPath)

	require.Equalf(t, expectedPointsTo, actualPointsTo, "symlink located at %s does not point to the expected path", symlinkPath)
}

func mustNotExist(t *testing.T, fsys FS, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := exists(fsys, path)
	require.NoError(err)
	require.False(found, "found file path but should not exist: "+path)
}

func mustNotLExist(t *testing.T, fsys FS, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := lExists(fsys, path)
	require.NoError(err)
	require.Falsef(found, "path found but should not exist: %s", path)
}

func mustExist(t *testing.T, fsys FS, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := exists(fsys, path)
	require.NoError(err)
	require.True(found, "file path not found but should exist: "+path)
}

func mustLExist(t *testing.T, fsys FS, path string) {
	path = filepath.Clean(path)

	require := require.New(t)
	found, err := lExists(fsys, path)
	require.NoError(err)
	require.Truef(found, "path not found but should exist: %s", path)
}

func removeFile(t *testing.T, fsys FS, path string) {
	path = filepath.Clean(path)

	require := require.New(t)

	err := fsys.Remove(path)
	require.NoError(err)

	mustNotLExist(t, fsys, path)
}

func removeAll(t *testing.T, fsys FS, path string) {
	path = filepath.Clean(path)

	require := require.New(t)

	err := fsys.RemoveAll(path)
	require.NoError(err)

	mustNotLExist(t, fsys, path)
}

func createSymlink(t *testing.T, fsys FS, oldpath, newpath string) {
	require := require.New(t)

	oldpath = filepath.Clean(oldpath)
	newpath = filepath.Clean(newpath)

	dirPath := filepath.Dir(oldpath)
	found, err := exists(fsys, toAbsSymlink(oldpath, newpath))
	require.NoError(err)

	if !found {
		err = fsys.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	dirPath = filepath.Dir(newpath)
	found, err = exists(fsys, dirPath)
	require.NoError(err)

	if !found {
		err = fsys.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	// check newpath after creating the symlink
	err = fsys.Symlink(oldpath, newpath)
	require.NoError(err)

	fi, err := fsys.Lstat(newpath)
	require.NoError(err)

	hasSymlinkFlag := fi.Mode()&os.ModeType&os.ModeSymlink != 0
	require.True(hasSymlinkFlag, "the target(newpath) symlink does not have the symlink flag set: ", newpath)

	// check oldpath after creating the symlink
	fi, err = fsys.Lstat(toAbsSymlink(oldpath, newpath))
	switch {
	case err == nil:
		hasSymlinkFlag = fi.Mode()&os.ModeType&os.ModeSymlink != 0
		require.Falsef(hasSymlinkFlag, "the source (oldpath) symlink does have the symlink flag set but is expected not to have it set: %s", oldpath)
	case errors.Is(err, fs.ErrNotExist):
		// broken symlink that points to an invalid location may be created
		return
	default:
		require.NoError(err)
	}

}

func createFile(t *testing.T, fsys FS, path, content string) {
	path = filepath.Clean(path)

	require := require.New(t)

	dirPath := filepath.Dir(path)
	found, err := exists(fsys, dirPath)
	require.NoError(err)

	if !found {
		err = fsys.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	f, err := fsys.Create(path)
	require.NoError(err)
	defer func(file File) {
		err := f.Close()
		require.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	require.NoError(err)
	require.Equal(ret, len(content))
}

func openFile(t *testing.T, fsys FS, path, content string, perm fs.FileMode) {
	path = filepath.Clean(path)

	require := require.New(t)

	dirPath := filepath.Dir(path)
	found, err := exists(fsys, dirPath)
	require.NoError(err)

	if !found {
		err = fsys.MkdirAll(dirPath, 0755)
		require.NoError(err)
	}

	f, err := fsys.OpenFile(path, os.O_RDWR|os.O_TRUNC|os.O_CREATE, perm)
	require.NoError(err)
	defer func(file File) {
		err := f.Close()
		require.NoError(err)
	}(f)
	ret, err := f.WriteString(content)
	require.NoError(err)
	require.Equal(ret, len(content))
}

func mkdirAll(t *testing.T, fsys FS, path string, perm fs.FileMode) {
	path = filepath.Clean(path)

	require := require.New(t)
	err := fsys.MkdirAll(path, perm)
	require.NoError(err)

	_, err = IterateDirTree(path, func(s string) (bool, error) {
		exists, err := exists(fsys, s)
		if err != nil {
			return false, err
		}
		require.True(exists, "path not found but is expected to exist: ", s)
		return true, nil
	})
	require.NoError(err)
}

func mkdir(t *testing.T, fsys FS, path string, perm fs.FileMode) error {
	path = filepath.Clean(path)

	require := require.New(t)
	err := fsys.Mkdir(path, perm)
	if err != nil {
		// assert that it is indeed a path error
		_, ok := err.(*os.PathError)
		require.True(ok)
		return err
	}

	b, err := lExists(fsys, path)
	require.NoError(err)
	require.True(b, "directory: ", path, "must exist after it has been created but does not.")
	return nil
}

func modeMustBeEqual(t *testing.T, a, b fs.FileMode) {
	require := require.New(t)

	a &= chmodBits
	b &= chmodBits

	require.Equalf(a, b, "expected: %0o got: %0o", a, b)
}

func chmod(t *testing.T, fsys FS, path string, perm fs.FileMode) {
	path = filepath.Clean(path)
	require := require.New(t)

	exists, err := lExists(fsys, path)
	require.NoError(err)

	if !exists {
		err = fsys.Chmod(path, perm)
		require.Error(err)
		return
	}

	// exists

	err = fsys.Chmod(path, perm)
	require.NoError(err)
	fiAfter, err := fsys.Lstat(path)
	require.NoError(err)

	permAfter := fiAfter.Mode()

	modeMustBeEqual(t, perm, permAfter)
}

func countFiles(t *testing.T, fsys FS, path string, expectedFilesAndDirs int) {
	require := require.New(t)
	path = filepath.Clean(path)

	files, err := allFiles(fsys, path)
	require.NoError(err)

	sort.Strings(files)

	require.Equalf(expectedFilesAndDirs, len(files), "files: %v", files)
}

func createFSState(t *testing.T, fsys FS, entrypoint string) []pathState {
	state, err := newFSState(fsys, entrypoint)
	require.NoError(t, err)
	return state
}

func mustEqualFSState(t *testing.T, before []pathState, fsys FS, entrypoint string) {
	after := createFSState(t, fsys, entrypoint)
	require.Equal(t, before, after)
}

func newFSState(fsys FS, entrypoint string) ([]pathState, error) {
	var paths []pathState
	err := Walk(fsys, entrypoint, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		content := ""
		if info.Mode().IsRegular() {
			f, err := fsys.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			b, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			content = string(b)
		} else if info.Mode()&os.ModeSymlink != 0 {
			content, err = fsys.Readlink(path)
			if err != nil {
				return err
			}
		}

		paths = append(paths, pathState{
			Path:    path,
			Name:    info.Name(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(byPathStateLeastFilePathSeparators(paths))
	return paths, nil
}

type pathState struct {
	Path    string
	Name    string
	Size    int64
	Mode    fs.FileMode
	Content string
}

type byPathStateLeastFilePathSeparators []pathState

func (a byPathStateLeastFilePathSeparators) Len() int      { return len(a) }
func (a byPathStateLeastFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byPathStateLeastFilePathSeparators) Less(i, j int) bool {
	return LessFilePathSeparators(a[i].Path, a[j].Path)
}
