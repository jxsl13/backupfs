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

	err := fs.Mkdir(name, info.Mode().Perm())
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

// ByStringLength sorts the string by th enumber of subdirectories
type ByFilePathSeparators []string

func (a ByFilePathSeparators) Len() int      { return len(a) }
func (a ByFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByFilePathSeparators) Less(i, j int) bool {
	ai := filepath.ToSlash(a[i])
	aj := filepath.ToSlash(a[j])

	return strings.Count(ai, "/") > strings.Count(aj, "/")
}
