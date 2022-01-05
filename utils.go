package backupfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/afero"
)

func copyDirAll(base afero.Fs, layer afero.Fs, name string) error {
	name = strings.TrimRight(name, string(filepath.Separator))

	create := false
	lastIndex := 0
	for i, r := range name {
		if i == 0 && r == filepath.Separator {
			continue
		}
		create = false

		if r == filepath.Separator {
			create = true
			lastIndex = i - 1
		} else if i == len(name)-1 {
			create = true
			lastIndex = i
		}

		if create {
			// /path -> /path/subpath -> /path/subpath/subsubpath etc.
			dirPath := name[:lastIndex]
			exists, err := dirExists(layer, dirPath)
			if err != nil {
				return err
			}

			if !exists {
				// does not exist in layer, must be created
				baseDir, err := base.Stat(dirPath)
				if err != nil {
					return err
				}
				err = layer.Mkdir(dirPath, baseDir.Mode().Perm())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func copyFile(base afero.Fs, layer afero.Fs, name string, bfh File) error {
	name = filepath.ToSlash(name)

	// First make sure the directory exists
	exists, err := exists(layer, filepath.Dir(name))
	if err != nil {
		return err
	}
	if !exists {
		err = copyDirAll(base, layer, filepath.Dir(name))
		if err != nil {
			return err
		}
	}

	// Create the file on the overlay
	lfh, err := layer.Create(name)
	if err != nil {
		return err
	}
	n, err := io.Copy(lfh, bfh)
	if err != nil {
		// If anything fails, clean up the file
		layer.Remove(name)
		lfh.Close()
		return err
	}

	bfi, err := bfh.Stat()
	if err != nil || bfi.Size() != n {
		layer.Remove(name)
		lfh.Close()
		return syscall.EIO
	}

	err = lfh.Close()
	if err != nil {
		layer.Remove(name)
		lfh.Close()
		return err
	}
	return layer.Chtimes(name, bfi.ModTime(), bfi.ModTime())
}

func copyToLayer(base afero.Fs, layer afero.Fs, name string) error {
	bfh, err := base.Open(name)
	if err != nil {
		return err
	}
	defer bfh.Close()

	return copyFile(base, layer, name, bfh)
}

func copyFileToLayer(base afero.Fs, layer afero.Fs, name string, flag int, perm os.FileMode) error {
	bfh, err := base.OpenFile(name, flag, perm)
	if err != nil {
		return err
	}
	defer bfh.Close()

	return copyFile(base, layer, name, bfh)
}

// dirExists checks if a path exists and is a directory.
func dirExists(fs afero.Fs, path string) (bool, error) {
	fi, err := fs.Stat(path)
	if err == nil && fi.IsDir() {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsDir checks if a given path is a directory.
func isDir(fs afero.Fs, path string) (bool, error) {
	fi, err := fs.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

// IsEmpty checks if a given file or directory is empty.
func isEmpty(fs afero.Fs, path string) (bool, error) {
	if b, _ := exists(fs, path); !b {
		return false, fmt.Errorf("%q path does not exist", path)
	}
	fi, err := fs.Stat(path)
	if err != nil {
		return false, err
	}
	if fi.IsDir() {
		f, err := fs.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()
		list, err := f.Readdir(-1)
		return len(list) == 0, err
	}
	return fi.Size() == 0, nil
}

// Check if a file or directory exists.
func exists(fs afero.Fs, path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
