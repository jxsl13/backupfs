package fsutils

import (
	"errors"
	"io/fs"
	"os"

	"github.com/jxsl13/backupfs/fsi"
)

// Check if a file or directory exists.
func Exists(ifs fsi.Fs, path string) (bool, error) {
	_, err := ifs.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// Check if a symlin, file or directory exists.
func LExists(fs fsi.Fs, path string) (bool, error) {

	_, err := fs.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err == nil {
		return true, nil
	}
	return false, err
}
