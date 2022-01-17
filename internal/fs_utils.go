package internal

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

var (
	ErrSymlinkInfoExpected = errors.New("expecting a symlink file-info")
	ErrDirInfoExpected     = errors.New("expecting a directory file-info")
	ErrFileInfoExpected    = errors.New("expecting a file file-info")
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

func CopyDir(fs afero.Fs, name string, info os.FileInfo) error {
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrDirInfoExpected, name)
	}

	// try to create all dirs as somone might have tempered with the file system
	err := fs.MkdirAll(name, info.Mode().Perm())
	if err != nil {
		return err
	}

	err = fs.Chmod(name, info.Mode())
	if err != nil {
		// TODO: do we want to fail here?
		return err
	}

	modTime := info.ModTime()
	err = fs.Chtimes(name, modTime, modTime)
	if err != nil {
		// TODO: do we want to fail here?
		return err
	}
	err = Chown(info, name, fs)
	if err != nil {
		return err
	}

	return nil
}

func CopyFile(fs afero.Fs, name string, info os.FileInfo, sourceFile afero.File) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrFileInfoExpected, name)
	}
	// same as create but with custom permissions
	file, err := fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
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

	err = fs.Chmod(name, info.Mode())
	if err != nil {
		return err
	}

	modTime := info.ModTime()
	err = fs.Chtimes(name, modTime, modTime)
	if err != nil {
		return err
	}
	err = Chown(info, name, fs)

	if err != nil {
		return err
	}

	return nil
}

type SymlinkerFs interface {
	afero.Fs
	afero.Symlinker
}

func CopySymlink(source, target afero.Fs, name string, info os.FileInfo, errBaseFsNoSymlink, errBackupFsNoSymlink error) error {

	if info.Mode().Type()&os.ModeSymlink == 0 {
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

	err = backupFs.Chmod(name, info.Mode())
	if err != nil {
		return err
	}

	modTime := info.ModTime()
	err = backupFs.Chtimes(name, modTime, modTime)
	if err != nil {
		return err
	}

	err = Chown(info, name, backupFs)
	if err != nil {
		return err
	}

	return nil
}

// Chown is an operating system dependent implementation.
func Chown(from fs.FileInfo, toName string, fs afero.Fs) error {

	err := fs.Chown(toName, Uid(from), Gid(from))
	if err != nil {
		return err
	}

	return nil
}

func RestoreFile(name string, backupFi fs.FileInfo, base, backup afero.Fs) error {
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

	if fi.IsDir() {
		// remove dir and create a file there
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

type LstaterFs interface {
	afero.Lstater
	afero.Fs
}

// Check if a symlin, file or directory exists.
func LExists(fs afero.Fs, path string) (bool, error) {
	lstater, ok := fs.(afero.Lstater)
	if !ok {
		return Exists(fs, path)
	}

	_, _, err := lstater.LstatIfPossible(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err == nil {
		return true, nil
	}
	return false, err
}

// Check if a symlin, file or directory exists.
func LStatIfPossible(fs afero.Fs, path string) (fs.FileInfo, bool, error) {
	lstater, ok := fs.(afero.Lstater)
	if !ok {
		fi, err := fs.Stat(path)
		if err == nil {
			return fi, false, nil
		}
		return nil, false, err
	}

	return lstater.LstatIfPossible(path)
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
