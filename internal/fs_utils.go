package internal

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jxsl13/backupfs/fsi"
	"github.com/jxsl13/backupfs/fsutils"
)

var (
	ErrSymlinkInfoExpected = errors.New("expecting a symlink file-info")
	ErrDirInfoExpected     = errors.New("expecting a directory file-info")
	ErrFileInfoExpected    = errors.New("expecting a file file-info")

	// internal package does not expose these errors
	ErrCopyFileFailed     = errors.New("failed to copy file")
	errWrapCopyFileFailed = func(err error) error {
		return fmt.Errorf("%w: %v", ErrCopyFileFailed, err)
	}

	ErrCopyDirFailed     = errors.New("failed to copy directory")
	errWrapCopyDirFailed = func(err error) error {
		return fmt.Errorf("%w: %v", ErrCopyDirFailed, err)
	}
)

// IgnorableChownError is solely used in Chown
func IgnorableChownError(err error) error {
	// first check os-specific ignorable errors, like on windoes not implemented
	err = ignorableChownError(err)
	if err == nil {
		return nil
	}

	// check is permission for chown is denied
	// if no permission for chown, we don't chown
	switch {
	case errors.Is(err, os.ErrPermission):
		return nil
	default:
		return err
	}
}

// IgnorableChtimesError is solely used for Chtimes
func IgnorableChtimesError(err error) error {
	err = ignorableChtimesError(err)
	if err == nil {
		return nil
	}

	// check is permission for chown is denied
	// if no permission for chown, we don't chtimes
	switch {
	case errors.Is(err, os.ErrPermission):
		return nil
	default:
		return err
	}
}

func CopyDir(name string, info fs.FileInfo, dst, src fsi.Fs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to copy directody: %s: %w", name, err)
		}
	}()
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrDirInfoExpected, name)
	}

	// try to create all dirs as somone might have tempered with the file system
	targetMode := info.Mode()
	err = dst.MkdirAll(name, targetMode.Perm())
	if err != nil {
		return err
	}

	newDirInfo, err := dst.Lstat(name)
	if err != nil {
		return errWrapCopyDirFailed(err)
	}

	currentMode := newDirInfo.Mode()

	if !EqualMode(currentMode, targetMode) {
		err = dst.Chmod(name, targetMode)
		if err != nil {
			// TODO: do we want to fail here?
			return err
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newDirInfo.ModTime()
	if !currentModTime.Equal(targetModTime) {
		err = dst.Chtimes(name, targetModTime, targetModTime)
		if err != nil {
			return err
		}
	}

	return Lchown(name, info, dst, src)
}

func CopyFile(name string, srcInfo fs.FileInfo, dst, src fsi.Fs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to copy file %s: %w", name, err)
		}
	}()

	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrFileInfoExpected, name)
	}
	targetMode := srcInfo.Mode()

	srcFile, err := src.Open(name)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// same as create operation but with custom permissions
	dstFile, err := dst.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, targetMode.Perm())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	err = Lchmod(name, srcInfo, dst, src)
	if err != nil {
		return err
	}

	err = Lchtimes(name, srcInfo, dst, src)
	if err != nil {
		return err
	}

	return Lchown(name, srcInfo, dst, src)
}

func CopySymlink(name string, srcInfo fs.FileInfo, dst, src fsi.Fs) error {

	if srcInfo.Mode()&fs.ModeType&fs.ModeSymlink == 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkInfoExpected, name)
	}

	pointsAt, err := src.Readlink(name)
	if err != nil {
		return err
	}

	err = dst.Symlink(pointsAt, name)
	if err != nil {
		return err
	}

	return Lchown(name, srcInfo, dst, src)
}

// Lchown is an operating system dependent implementation.
// only tries to change owner in case that the owner differs
func Lchown(name string, info fs.FileInfo, dst, src fsi.Fs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to apply chown to %s: %w", name, err)
		}
	}()

	uid, gid, err := src.Lown(name)
	if err != nil {
		return err
	}

	cuid, cgid, err := dst.Lown(name)
	if err != nil {
		return err
	}

	if uid != cuid || gid != cgid {
		return dst.Lchown(name, uid, gid)
	}
	return nil
}

func Lchmod(name string, srcInfo fs.FileInfo, dst, src fsi.Fs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to apply chmod to %s: %w", name, err)
		}
	}()

	dstInfo, err := dst.Lstat(name)
	if err != nil {
		return err
	}

	if EqualMode(dstInfo.Mode(), srcInfo.Mode()) {
		// nothing to do
		return nil
	}

	return dst.Chmod(name, srcInfo.Mode())
}

func Lchtimes(name string, srcInfo fs.FileInfo, dst, src fsi.Fs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to apply chtimes to %s: %w", name, err)
		}
	}()

	if srcInfo == nil {
		srcInfo, err = src.Lstat(name)
		if err != nil {
			return err
		}
	}

	if srcInfo.Mode()&fs.ModeSymlink != 0 {
		// is symlink -> nothing to do
		return nil
	}

	dstInfo, err := dst.Lstat(name)
	if err != nil {
		return err
	}

	if dstInfo.Mode()&fs.ModeSymlink != 0 {
		// should not happen
		// is symlink -> nothing to do
		return nil
	}

	var (
		srcTime = srcInfo.ModTime()
		dstTime = dstInfo.ModTime()
	)

	if srcTime.Equal(dstTime) {
		// nothing to do
		return nil
	}

	return dst.Chtimes(name, srcTime, srcTime)
}

func RestoreFile(name string, srcInfo fs.FileInfo, dst, src fsi.Fs) error {
	fi, err := src.Stat(name)
	if err != nil {
		// best effort, see above
		return nil
	}

	if !fi.Mode().IsRegular() {
		// remove dir/symlink/etc and create a file there
		err = dst.RemoveAll(name)
		if err != nil {
			// we failed to remove the directory
			// supposedly we cannot restore the file, as the directory still exists
			return nil
		}
	}

	// in case that the application does not hold any backup data in memory anymore
	// we fallback to using the file permissions of the actual backed up file
	if srcInfo != nil {
		fi = srcInfo
	}

	// move file back to base system
	err = CopyFile(name, srcInfo, dst, src)
	if err != nil {
		// failed to restore file
		// critical error, most likely due to network problems
		return err
	}
	return nil
}

func RestoreSymlink(name string, srcInfo fs.FileInfo, dst, src fsi.Fs) error {
	exists, err := fsutils.LExists(src, name)
	if err != nil || !exists {
		// best effort, if src broken, we cannot restore
		return nil
	}

	newFileExists, err := fsutils.LExists(dst, name)
	if err == nil && newFileExists {
		// remove dir/symlink/etc and create a new symlink there
		err = dst.RemoveAll(name)
		if err != nil {
			// in case we fail to remove the new file,
			// we cannot restore the symlink
			// best effort, fail silently
			return nil
		}
	}

	// try to restore symlink
	return CopySymlink(name, srcInfo, dst, src)
}

// current OS filepath separator / or \
const separator = string(filepath.Separator)

// ByMostFilePathSeparators sorts the string by the number of file path separators
// the more nested this is, the further at the beginning of the string slice the path will be
type ByMostFilePathSeparators []string

func (a ByMostFilePathSeparators) Len() int      { return len(a) }
func (a ByMostFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByMostFilePathSeparators) Less(i, j int) bool {

	return strings.Count(a[i], separator) > strings.Count(a[j], separator)
}

// ByLeastFilePathSeparators sorts the string by the number of file path separators
// the least nested the file path is, the further at the beginning it will be of the
// sorted string slice.
type ByLeastFilePathSeparators []string

func (a ByLeastFilePathSeparators) Len() int      { return len(a) }
func (a ByLeastFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByLeastFilePathSeparators) Less(i, j int) bool {

	return strings.Count(a[i], separator) < strings.Count(a[j], separator)
}
