package internal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
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

func IterateDirTree(name string, visitor func(string) error) error {
	name = filepath.Clean(name)

	create := false
	lastIndex := 0
	for i, r := range name {
		if i == 0 && r == filepath.Separator {
			continue
		}
		create = false

		if r == '/' {
			create = true
			lastIndex = i
		}
		if i == len(name)-1 {
			create = true
			lastIndex = i + 1
		}

		if create {
			// /path -> /path/subpath -> /path/subpath/subsubpath etc.
			dirPath := name[:lastIndex]
			err := visitor(dirPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

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

func CopyDir(fs afero.Fs, name string, info os.FileInfo) error {
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrDirInfoExpected, name)
	}

	// try to create all dirs as somone might have tempered with the file system
	targetMode := info.Mode()
	err := fs.MkdirAll(name, targetMode.Perm())
	if err != nil {
		return err
	}

	newDirInfo, _, err := LstatIfPossible(fs, name)
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

	// https://pkg.go.dev/os#Chown
	// Windows & Plan9 not supported
	err = IgnorableChownError(Chown(info, name, fs))
	if err != nil {
		return err
	}

	return nil
}

func CopyFile(fs afero.Fs, name string, info os.FileInfo, sourceFile afero.File) error {
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

	newFileInfo, _, err := LstatIfPossible(fs, name)
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

	// might cause a windows error that this function is not implemented by the OS
	// in a unix fassion
	// permission and not implemented errors are ignored
	err = IgnorableChownError(Chown(info, name, fs))
	if err != nil {
		return errWrapCopyFileFailed(err)
	}

	return nil
}

type SymlinkerFs interface {
	afero.Fs
	afero.Symlinker
	LinkOwner
}

func CopySymlink(source, target afero.Fs, name string, info os.FileInfo, errBaseFsNoSymlink, errBackupFsNoSymlink error) error {

	if info.Mode()&os.ModeType&os.ModeSymlink == 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkInfoExpected, name)
	}

	baseFs, ok := source.(SymlinkerFs)
	if !ok {
		return errBaseFsNoSymlink
	}

	backupFs, ok := target.(SymlinkerFs)
	if !ok {
		return errBackupFsNoSymlink
	}

	pointsAt, err := baseFs.ReadlinkIfPossible(name)
	if err != nil {
		return err
	}

	err = backupFs.SymlinkIfPossible(pointsAt, name)
	if err != nil {
		return err
	}

	return IgnorableChownError(backupFs.LchownIfPossible(name, Uid(info), Gid(info)))
}

// Chown is an operating system dependent implementation.
// only tries to change owner in cas ethat the owner differs
func Chown(from os.FileInfo, toName string, fs afero.Fs) error {

	oldOwnerFi, _, err := LstatIfPossible(fs, toName)
	if err != nil {
		return fmt.Errorf("lstat for chown failed: %w", err)
	}

	oldUid := Uid(oldOwnerFi)
	oldGid := Gid(oldOwnerFi)

	newUid := Uid(from)
	newGid := Gid(from)

	// only update when something changed
	if oldUid != newUid || oldGid != newGid {
		err = fs.Chown(toName, Uid(from), Gid(from))
		if err != nil {
			return err
		}
	}
	return nil
}

func RestoreFile(name string, backupFi os.FileInfo, base, backup afero.Fs) error {
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

func RestoreSymlink(name string, backupFi os.FileInfo, base, backup afero.Fs, errBaseFsNoSymlink, errBackupFsNoSymlink error) error {
	exists, err := LExists(backup, name)
	if err != nil || !exists {
		// best effort, if backup broken, we cannot restore
		return nil
	}

	newFileExists, err := LExists(base, name)
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
	return CopySymlink(backup, base, name, backupFi, errBaseFsNoSymlink, errBackupFsNoSymlink)
}

// Check if a file or directory exists.
func Exists(fs afero.Fs, path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Check if a symlin, file or directory exists.
func LExists(fs afero.Fs, path string) (bool, error) {
	lstater, ok := fs.(afero.Lstater)
	if !ok {
		return Exists(fs, path)
	}

	_, _, err := lstater.LstatIfPossible(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err == nil {
		return true, nil
	}
	return false, err
}

// Check if interface is implemented
func LstaterIfPossible(fs afero.Fs) (afero.Lstater, bool) {
	lstater, ok := fs.(afero.Lstater)
	if ok {
		return lstater, true
	}
	return nil, false
}

// LstatIfPossible uses Lstat or stat in case it is possible.
// returns the fileinfo of the symlink or of the linked file or of the file in
// case there is no symlink. The second return value returns true in case Lstat
// was actually called, false otherwise.
func LstatIfPossible(fs afero.Fs, path string) (os.FileInfo, bool, error) {
	lstater, ok := fs.(afero.Lstater)
	if ok {
		fi, b, err := lstater.LstatIfPossible(path)
		if fi == nil {
			return nil, b, err
		}
		return fi, b, nil
	}
	fi, err := fs.Stat(path)
	if fi == nil {
		return nil, false, err
	}

	return fi, false, nil
}

// tries lchown, does not guarantee success
func LchownIfPossible(fs afero.Fs, name string, uid, gid int) error {
	linkOwner, ok := fs.(LinkOwner)
	if !ok {
		return nil
	}

	return linkOwner.LchownIfPossible(name, uid, gid)
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
