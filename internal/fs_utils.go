package internal

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/afero"
)

func IterateDirTree(name string, visitor func(string) error) error {
	name = filepath.Clean(name)
	slashName := filepath.ToSlash(name)

	create := false
	lastIndex := 0
	for i, r := range slashName {
		if i == 0 && r == '/' {
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
		panic("expecting a directory file-info")
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

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid := int(stat.Uid)
		gid := int(stat.Gid)

		err = fs.Chown(name, uid, gid)
		if err != nil {
			// TODO: do we want to fail here?
			return err
		}
	}
	return nil
}

func CopyFile(fs afero.Fs, name string, info os.FileInfo, sourceFile afero.File) error {
	if info.IsDir() {
		panic("expecting a file file-info")
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

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid := int(stat.Uid)
		gid := int(stat.Gid)

		err = fs.Chown(name, uid, gid)
		if err != nil {
			return err
		}
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

// ByMostFilePathSeparators sorts the string by the number of file path separators
// the more nested this is, the further at the beginning of the string slice the path will be
type ByMostFilePathSeparators []string

func (a ByMostFilePathSeparators) Len() int      { return len(a) }
func (a ByMostFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByMostFilePathSeparators) Less(i, j int) bool {
	ai := filepath.ToSlash(a[i])
	aj := filepath.ToSlash(a[j])

	return strings.Count(ai, "/") > strings.Count(aj, "/")
}

// ByLeastFilePathSeparators sorts the string by the number of file path separators
// the least nested the file path is, the further at the beginning it will be of the
// sorted string slice.
type ByLeastFilePathSeparators []string

func (a ByLeastFilePathSeparators) Len() int      { return len(a) }
func (a ByLeastFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByLeastFilePathSeparators) Less(i, j int) bool {
	ai := filepath.ToSlash(a[i])
	aj := filepath.ToSlash(a[j])

	return strings.Count(ai, "/") < strings.Count(aj, "/")
}
