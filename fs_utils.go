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
	"syscall"
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

func copyDir(fsys FS, name string, info fs.FileInfo) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %s: %v", errCopyDirFailed, name, err)
		}
	}()

	if !info.IsDir() {
		return fmt.Errorf("%w: %s", errDirInfoExpected, name)
	}

	// do not touch the root directory
	// this is either the OS root directory, which we do not want to change, as
	// on for example redhat it's a read only directory which is not modifiable.
	// on the other hand it is the root directory of the backup folder which has already its permissions
	// set correctly.
	pathWithoutVolume := TrimVolume(name)
	if pathWithoutVolume == separator || pathWithoutVolume == "/" {
		// windows supports both path separators, which is why we check for both
		return nil
	}

	// try to create all dirs as somone might have tempered with the file system
	targetMode := info.Mode() & (fs.ModeSticky | fs.ModePerm)
	err = fsys.MkdirAll(name, targetMode)
	if err != nil {
		return err
	}

	newDirInfo, err := fsys.Lstat(name)
	if err != nil {
		return fmt.Errorf("%w: %v", errCopyDirFailed, err)
	}

	currentMode := newDirInfo.Mode()

	if !equalMode(currentMode, targetMode) {
		err = fsys.Chmod(name, targetMode)
		if err != nil {
			// TODO: do we want to fail here?
			return err
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newDirInfo.ModTime()
	if !currentModTime.Equal(targetModTime) {
		err = ignoreChtimesError(fsys.Chtimes(name, targetModTime, targetModTime))
		if err != nil {
			return err
		}
	}

	// https://pkg.go.dev/os#Chown
	// Windows & Plan9 not supported
	err = ignoreChownError(chown(info, name, fsys))
	if err != nil {
		return err
	}

	return nil
}

func copyFile(fsys FS, name string, info fs.FileInfo, sourceFile File) (err error) {
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

	err = writeFile(fsys, name, targetMode.Perm(), sourceFile)
	if err != nil {
		return err
	}

	newFileInfo, err := fsys.Lstat(name)
	if err != nil {
		return err
	}

	if !equalMode(newFileInfo.Mode(), targetMode) {
		// not equal, update it
		err = fsys.Chmod(name, targetMode)
		if err != nil {
			return err
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newFileInfo.ModTime()

	if !currentModTime.Equal(targetModTime) {
		err = ignoreChtimesError(fsys.Chtimes(name, targetModTime, targetModTime))
		if err != nil {
			return err
		}
	}

	// might cause a windows error that this function is not implemented by the OS
	// in a unix fassion
	// permission and not implemented errors are ignored
	err = ignoreChownError(chown(info, name, fsys))
	if err != nil {
		return err
	}

	return nil
}

func writeFile(fsys FS, name string, perm fs.FileMode, content io.Reader) (err error) {
	// same as create but with custom permissions
	file, err := fsys.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm.Perm())
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	_, err = io.Copy(file, content)
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
func chown(from fs.FileInfo, toName string, fsys FS) error {

	oldOwnerFi, err := fsys.Lstat(toName)
	if err != nil {
		return fmt.Errorf("lstat for chown failed: %w", err)
	}

	oldUid := toUID(oldOwnerFi)
	oldGid := toGID(oldOwnerFi)

	newUid := toUID(from)
	newGid := toGID(from)

	// only update when something changed
	if oldUid != newUid || oldGid != newGid {
		err = fsys.Chown(toName, toUID(from), toGID(from))
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

	_, exists, err := lexists(backup, name)
	if err != nil || !exists {
		// best effort, if backup broken, we cannot restore
		return nil
	}

	_, newFileExists, err := lexists(base, name)
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

// Check if a symlin, file or directory exists.
func lexists(fsys FS, path string) (fs.FileInfo, bool, error) {
	fi, err := fsys.Lstat(path)
	if isNotFoundError(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	return fi, true, nil
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
//func toRelSymlink(oldname, newname string) (string, error) {
//	if isAbs(oldname) {
//		return filepath.Rel(filepath.Dir(newname), oldname)
//	}
//	return oldname, nil
//}

func isAbs(name string) bool {
	return path.IsAbs(filepath.ToSlash(name)) || filepath.IsAbs(filepath.FromSlash(name))
}

type resolverFS interface {
	Lstat(name string) (fs.FileInfo, error)
	Readlink(name string) (string, error)
}

func resolvePath(fsys resolverFS, filePath string) (resolvedFilePath string, err error) {
	resolvedFilePath, _, err = resolvePathWithInfo(fsys, filePath)
	return resolvedFilePath, err
}

func resolvePathWithFound(fsys resolverFS, filePath string) (resolvedFilePath string, found bool, err error) {
	resolvedFilePath, fi, err := resolvePathWithInfo(fsys, filePath)
	return resolvedFilePath, fi != nil, err
}

// resolvePath resolves a path that contains symlinks.
// The returned path is the resolved path.
// In case that the returned path is not equal to the path that was passed to this function
// then there was a symlink somewhere along the way to that file or directory.
// WARNING: The last element of the path is NOT resolved.
// Returns the file info of the last unresolved element.
// In case that the file path was not found, the returned FileInfo is nil.
func resolvePathWithInfo(fsys resolverFS, filePath string) (resolvedFilePath string, fi fs.FileInfo, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to resolve path: %s: %w", filePath, err)
		}
	}()

	if filePath == "" {
		return "", nil, errors.New("empty file path")
	}

	accPaths := make([]string, 0, strings.Count(filePath, separator))
	// collect all subdir segments
	_, _ = IterateDirTree(filePath, func(subdirPath string) (bool, error) {
		accPaths = append(accPaths, subdirPath)
		return true, nil
	})

	// do not use range here
	for i := 0; i < len(accPaths); i++ {
		p := accPaths[i]

		// iterate over all accumulated path segments /a -> /a/b  -> /a/b/c.txt etc.
		fi, err = fsys.Lstat(p)
		if err != nil {
			if isNotFoundError(err) {

				// return current resolved path state even if it was not found
				// e.g. /a/symlink/test.txt with /a/symlink pointing to /a/folder, then the resolved nam ewill be /a/folder/test.txt
				return accPaths[len(accPaths)-1], nil, nil
			}
			return "", nil, err
		}

		// check if symlink
		if fi.Mode()&os.ModeSymlink != 0 {
			// resolve symlink
			linkedPath, err := fsys.Readlink(p)
			if err != nil {
				return "", nil, err
			}
			linkedPath = toAbsSymlink(linkedPath, p)

			// update slice in place for all following paths after the symlink
			replacePathPrefix(accPaths[i+1:], p, linkedPath)
		}
	}

	return accPaths[len(accPaths)-1], fi, nil
}

func replacePathPrefix(paths []string, oldPrefix, newPrefix string) {
	for idx, path := range paths {
		paths[idx] = filepath.Join(newPrefix, strings.TrimPrefix(path, oldPrefix))
	}
}

func isNotFoundError(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTDIR)
}
