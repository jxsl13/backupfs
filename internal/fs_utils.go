package internal

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jxsl13/backupfs/fsutils"
	"github.com/jxsl13/backupfs/interfaces"
	"github.com/jxsl13/backupfs/osutils"
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

func CopyDir(fs interfaces.Fs, name string, info fs.FileInfo) error {
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrDirInfoExpected, name)
	}

	// try to create all dirs as somone might have tempered with the file system
	targetMode := info.Mode()
	err := fs.MkdirAll(name, targetMode.Perm())
	if err != nil {
		return err
	}

	newDirInfo, err := fs.Lstat(name)
	if err != nil {
		return errWrapCopyDirFailed(err)
	}

	currentMode := newDirInfo.Mode()

	if !EqualMode(currentMode, targetMode) {
		err = fs.Chmod(name, targetMode)
		if err != nil {
			// TODO: do we want to fail here?
			return err
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newDirInfo.ModTime()
	if !currentModTime.Equal(targetModTime) {
		err = IgnorableChtimesError(fs.Chtimes(name, targetModTime, targetModTime))
		if err != nil {
			return err
		}
	}

	err = Chown(info, name, fs)
	if err != nil {
		return err
	}

	return nil
}

func CopyFile(fs interfaces.Fs, name string, info fs.FileInfo, sourceFile interfaces.File) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrFileInfoExpected, name)
	}
	//
	targetMode := info.Mode()

	// same as create but with custom permissions
	file, err := fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, targetMode.Perm())
	if err != nil {
		return errWrapCopyFileFailed(err)
	}

	_, err = io.Copy(file, sourceFile)
	if err != nil {
		return errWrapCopyFileFailed(err)
	}

	err = file.Close()
	if err != nil {
		return errWrapCopyFileFailed(err)
	}

	newFileInfo, err := fs.Lstat(name)
	if err != nil {
		return errWrapCopyFileFailed(err)
	}

	if !EqualMode(newFileInfo.Mode(), targetMode) {
		// not equal, update it
		err = fs.Chmod(name, targetMode)
		if err != nil {
			return errWrapCopyFileFailed(err)
		}
	}

	targetModTime := info.ModTime()
	currentModTime := newFileInfo.ModTime()

	if !currentModTime.Equal(targetModTime) {
		err = IgnorableChtimesError(fs.Chtimes(name, targetModTime, targetModTime))
		if err != nil {
			return errWrapCopyFileFailed(err)
		}
	}

	/*
		// TODO: we will need to fix the file owner at some point
		// might cause a windows error that this function is not implemented by the OS
		// in a unix fassion
		// permission and not implemented errors are ignored
		err = IgnorableChownError(Chown(info, name, fs))
		if err != nil {
			return errWrapCopyFileFailed(err)
		}
	*/

	return nil
}

func CopySymlink(source, target interfaces.Fs, name string, info fs.FileInfo) error {

	if info.Mode()&fs.ModeType&fs.ModeSymlink == 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkInfoExpected, name)
	}

	pointsAt, err := source.Readlink(name)
	if err != nil {
		return err
	}

	err = target.Symlink(pointsAt, name)
	if err != nil {
		return err
	}
	return nil

	/*
		// TODO: might need to fix file owner
		user, err := OwnerUser(name, info)
		if err != nil {
			return err
		}
		group, err := OwnerGroup(name, info)
		if err != nil {
			return err
		}

		return target.Lchown(name, user, group)
	*/
}

// Chown is an operating system dependent implementation.
// only tries to change owner in case that the owner differs
func Chown(from fs.FileInfo, toName string, fs interfaces.Fs) error {

	oldOwnerFi, err := fs.Lstat(toName)
	if err != nil {
		return fmt.Errorf("lstat for chown failed: %w", err)
	}

	oldUser, err := osutils.OwnerGroup(toName, oldOwnerFi)
	if err != nil {
		return err
	}
	oldGroup, err := Groupname(toName, oldOwnerFi)
	if err != nil {
		return err
	}

	newUser, err := Username(toName, oldOwnerFi)
	if err != nil {
		return err
	}
	newGroup, err := Groupname(toName, oldOwnerFi)
	if err != nil {
		return err
	}

	// only update when something changed
	if oldUser != newUser || oldGroup != newGroup {
		err = fs.Chown(toName, newUser, newGroup)
		if err != nil {
			return err
		}
	}
	return nil
}

func RestoreFile(name string, backupFi fs.FileInfo, base, backup interfaces.Fs) error {
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
	err = CopyFile(base, name, backupFi, f)
	if err != nil {
		// failed to restore file
		// critical error, most likely due to network problems
		return err
	}
	return nil
}

func RestoreSymlink(name string, backupFi fs.FileInfo, base, backup interfaces.Fs) error {
	exists, err := fsutils.LExists(backup, name)
	if err != nil || !exists {
		// best effort, if backup broken, we cannot restore
		return nil
	}

	newFileExists, err := fsutils.LExists(base, name)
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
	return CopySymlink(backup, base, name, backupFi)
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
