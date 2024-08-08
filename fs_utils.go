package backupfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	errSymlinkInfoExpected = errors.New("expecting a symlink file-info")
	errDirInfoExpected     = errors.New("expecting a directory file-info")
	errFileInfoExpected    = errors.New("expecting a file file-info")

	// internal package does not expose these errors
	errCopyFileFailed    = errors.New("failed to copy file")
	errCopyDirFailed     = errors.New("failed to copy directory")
	errCopySymlinkFailed = errors.New("failed to copy symlink")
)

// / -> /a -> /a/b -> /a/b/c -> /a/b/c/d
// IterateDirTree does not clean the passed file name.
func IterateDirTree(name string, visitor func(string) (proceed bool, err error)) (aborted bool, err error) {

	var (
		create    = false
		lastIndex = 0
		proceed   = true
	)
	for i, r := range name {
		create = false

		if r == '/' || r == filepath.Separator {
			create = true
			lastIndex = max(i, 1) // root element should be visible
		}
		if i == len(name)-1 {
			create = true
			lastIndex = i + 1
		}

		if create {
			// /path -> /path/subpath -> /path/subpath/subsubpath etc.
			dirPath := name[:lastIndex]
			proceed, err = visitor(dirPath)
			if err != nil {
				return false, err
			}
			if !proceed {
				return true, nil
			}
		}
	}

	return false, nil
}

// ignoreChownError is solely used in Chown
func ignoreChownError(err error) error {
	// first check os-specific ignorable errors, like on windoes not implemented
	err = ignorableChownError(err)

	// check is permission for chown is denied
	// if no permission for chown, we don't chown
	switch {
	case errors.Is(err, fs.ErrPermission):
		return nil
	default:
		return err
	}
}

// ignoreChtimesError is solely used for Chtimes
func ignoreChtimesError(err error) error {
	err = ignorableChtimesError(err)
	if err == nil {
		return nil
	}

	// check is permission for chown is denied
	// if no permission for chown, we don't chtimes
	switch {
	case errors.Is(err, fs.ErrPermission):
		return nil
	default:
		return err
	}
}

func copyDir(fs FS, name string, info fs.FileInfo) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %s: %v", errCopyDirFailed, name, err)
		}
	}()

	if !info.IsDir() {
		return fmt.Errorf("%w: %s", errDirInfoExpected, name)
	}

	// try to create all dirs as somone might have tempered with the file system
	targetMode := info.Mode()
	err = fs.MkdirAll(name, targetMode.Perm())
	if err != nil {
		return err
	}

	newDirInfo, err := fs.Lstat(name)
	if err != nil {
		return fmt.Errorf("%w: %v", errCopyDirFailed, err)
	}

	currentMode := newDirInfo.Mode()

	if !equalMode(currentMode, targetMode) {
		err = fs.Chmod(name, targetMode)
		if err != nil {
			// TODO: do we want to fail here?
			return err
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newDirInfo.ModTime()
	if !currentModTime.Equal(targetModTime) {
		err = ignoreChtimesError(fs.Chtimes(name, targetModTime, targetModTime))
		if err != nil {
			return err
		}
	}

	// https://pkg.go.dev/os#Chown
	// Windows & Plan9 not supported
	err = ignoreChownError(chown(info, name, fs))
	if err != nil {
		return err
	}

	return nil
}

func copyFile(fs FS, name string, info fs.FileInfo, sourceFile File) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %s: %v", errCopyFileFailed, name, err)
		}
	}()

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", errFileInfoExpected, name)
	}
	//
	targetMode := info.Mode()

	// same as create but with custom permissions
	file, err := fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, targetMode.Perm())
	if err != nil {
		return err
	}

	_, err = io.Copy(file, sourceFile)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	newFileInfo, err := fs.Lstat(name)
	if err != nil {
		return err
	}

	if !equalMode(newFileInfo.Mode(), targetMode) {
		// not equal, update it
		err = fs.Chmod(name, targetMode)
		if err != nil {
			return err
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newFileInfo.ModTime()

	if !currentModTime.Equal(targetModTime) {
		err = ignoreChtimesError(fs.Chtimes(name, targetModTime, targetModTime))
		if err != nil {
			return err
		}
	}

	// might cause a windows error that this function is not implemented by the OS
	// in a unix fassion
	// permission and not implemented errors are ignored
	err = ignoreChownError(chown(info, name, fs))
	if err != nil {
		return err
	}

	return nil
}

func copySymlink(source, target FS, name string, info fs.FileInfo) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %s: %v", errCopySymlinkFailed, name, err)
		}
	}()

	if info.Mode()&os.ModeType&os.ModeSymlink == 0 {
		return fmt.Errorf("%w: %s", errSymlinkInfoExpected, name)
	}

	pointsAt, err := source.Readlink(name)
	if err != nil {
		return err
	}

	err = target.Symlink(pointsAt, name)
	if err != nil {
		return err
	}

	return ignoreChownError(target.Lchown(name, toUID(info), toGID(info)))
}

// Chown is an operating system dependent implementation.
// only tries to change owner in cas ethat the owner differs
func chown(from fs.FileInfo, toName string, fs FS) error {

	oldOwnerFi, err := fs.Lstat(toName)
	if err != nil {
		return fmt.Errorf("lstat for chown failed: %w", err)
	}

	oldUid := toUID(oldOwnerFi)
	oldGid := toGID(oldOwnerFi)

	newUid := toUID(from)
	newGid := toGID(from)

	// only update when something changed
	if oldUid != newUid || oldGid != newGid {
		err = fs.Chown(toName, toUID(from), toGID(from))
		if err != nil {
			return err
		}
	}
	return nil
}

func restoreFile(name string, backupFi fs.FileInfo, base, backup FS) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to restore file: %s: %w", name, err)
		}
	}()
	f, err := backup.Open(name)
	if err != nil {
		// best effort, if backup was tempered with, we cannot restore the file.
		return nil
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		// best effort, see above
		return nil
	}

	if !fi.Mode().IsRegular() {
		// remove dir/symlink/etc and create a file there
		err = base.RemoveAll(name)
		if err != nil {
			// we failed to remove the directory
			// supposedly we cannot restore the file, as the directory still exists
			return nil
		}
	}

	// in case that the application dooes not hold any backup data in memory anymore
	// we fallback to using the file permissions of the actual backed up file
	if backupFi != nil {
		fi = backupFi
	}

	// move file back to base system
	err = copyFile(base, name, backupFi, f)
	if err != nil {
		// failed to restore file
		// critical error, most likely due to network problems
		return err
	}
	return nil
}

func restoreSymlink(name string, backupFi fs.FileInfo, base, backup FS) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to restore symlink: %s: %w", name, err)
		}
	}()

	exists, err := lExists(backup, name)
	if err != nil || !exists {
		// best effort, if backup broken, we cannot restore
		return nil
	}

	newFileExists, err := lExists(base, name)
	if err == nil && newFileExists {
		// remove dir/symlink/etc and create a new symlink there
		err = base.RemoveAll(name)
		if err != nil {
			// in case we fail to remove the new file,
			// we cannot restore the symlink
			// best effort, fail silently
			return nil
		}
	}

	// try to restore symlink
	return copySymlink(backup, base, name, backupFi)
}

// Check if a file or directory exists.
func exists(fsys FS, path string) (bool, error) {
	_, err := fsys.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// Check if a symlin, file or directory exists.
func lExists(fsys FS, path string) (bool, error) {
	_, err := fsys.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

// equalMode is os-Dependent
func equalMode(a, b fs.FileMode) bool {
	// mask with os-specific masks
	a &= chmodBits
	b &= chmodBits

	return a == b
}

// toAbsSymlink always returns the absolute path to a symlink.
// newname is the symlink location, oldname is the location that
// the symlink is supposed point at. If oldname is a relative path,
// then the absolute path is calculated and returned instead.
func toAbsSymlink(oldname, newname string) string {
	if !isAbs(oldname) {
		return filepath.Join(filepath.Dir(newname), oldname)
	}
	return oldname
}

// toRelSymlink always returns the relative path to a symlink.
// newname is the symlink location, oldname is the location that
// the symlink is supposed point at. If oldname is an absolute path,
// then the relative path is calculated and returned instead.
func toRelSymlink(oldname, newname string) (string, error) {
	if isAbs(oldname) {
		return filepath.Rel(filepath.Dir(newname), oldname)
	}
	return oldname, nil
}

func isAbs(name string) bool {
	return path.IsAbs(filepath.ToSlash(name)) || filepath.IsAbs(filepath.FromSlash(name))
}

// resolvePath resolves a path that contains symlinks.
// The returned path is the resolved path.
// In case that the returned path is not equal to the path that was passed to this function
// then there was a symlink somewhere along the way to that file or directory.
func resolvePath(fsys FS, filePath string) (resolvedFilePath string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to resolve path: %s: %w", filePath, err)
		}
	}()

	if filePath == "" {
		return "", errors.New("empty file path")
	}

	accPaths := make([]string, 0, strings.Count(filePath, separator))
	// collect all subdir segmrents
	_, _ = IterateDirTree(filePath, func(subdirPath string) (bool, error) {
		accPaths = append(accPaths, subdirPath)
		return true, nil
	})

	for i, p := range accPaths {
		// iterate over all accumulated path segments /a -> /a/b  -> /a/b/c.txt etc.
		fi, err := fsys.Lstat(p)
		if err != nil {
			return "", err
		}

		// check if symlink
		if fi.Mode()&os.ModeSymlink != 0 {
			// resolve symlink
			linkedPath, err := fsys.Readlink(p)
			if err != nil {
				return "", err
			}
			linkedPath = toAbsSymlink(linkedPath, p)

			// update slice in place for all following paths after the symlink
			replacePathPrefix(accPaths[i+1:], p, linkedPath)
		}
	}

	return accPaths[len(accPaths)-1], nil
}

func resolvePathWithCache(fsys FS, filePath string, resolvedFileInfos map[string]fs.FileInfo, resolvedPathCache map[string]string) (resolvedPath string, err error) {

	var ok bool
	if resolvedPath, ok = resolvedPathCache[filePath]; ok {
		return resolvedPath, nil
	}

	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to resolve path: %s: %w", filePath, err)
		}
	}()

	if filePath == "" {
		return "", errors.New("empty file path")
	}

	accPaths := make([]string, 0, strings.Count(filePath, separator))
	// collect all subdir segmrents
	_, _ = IterateDirTree(filePath, func(subdirPath string) (bool, error) {
		accPaths = append(accPaths, subdirPath)
		return true, nil
	})

	var (
		fi         fs.FileInfo
		linkedPath string
	)

	for i, p := range accPaths {
		// skip readlink if already known
		linkedPath, ok = resolvedPathCache[p]
		if ok {
			if linkedPath != p {
				// update slice in place for all following paths after the symlink
				replacePathPrefix(accPaths[i+1:], p, linkedPath)
			}
			continue
		}

		// file path not yet resolved

		// skip lstat if file is already known
		fi, ok = resolvedFileInfos[p]
		if !ok {
			// iterate over all accumulated path segments /a -> /a/b  -> /a/b/c.txt etc.
			fi, err = fsys.Lstat(p)
			if err != nil {
				return "", err
			}
			resolvedFileInfos[p] = fi
		}

		// check if symlink
		if fi.Mode()&os.ModeSymlink != 0 {

			// skip readlink if already known
			linkedPath, ok = resolvedPathCache[p]
			if !ok {
				// resolve symlink
				linkedPath, err = fsys.Readlink(p)
				if err != nil {
					return "", err
				}
				linkedPath = toAbsSymlink(linkedPath, p)
				resolvedPathCache[p] = linkedPath
			}

			// update slice in place for all following paths after the symlink
			replacePathPrefix(accPaths[i+1:], p, linkedPath)
		} else {
			resolvedPathCache[p] = p
		}
	}

	resolvedPath = accPaths[len(accPaths)-1]
	if resolvedPath != filePath {
		resolvedPathCache[filePath] = resolvedPath
	}

	return resolvedPath, nil
}

func replacePathPrefix(paths []string, oldPrefix, newPrefix string) {
	for idx, path := range paths {
		paths[idx] = filepath.Join(newPrefix, strings.TrimPrefix(path, oldPrefix))
	}
}
