package internal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

func IsHidden(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}
	// reference: https://stackoverflow.com/questions/28024731/check-if-given-path-is-a-subdirectory-of-another-in-golang?rq=1

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(filepath.ToSlash(name))

	for _, hiddenDir := range hiddenPaths {

		relPath, err := filepath.Rel(hiddenDir, name)
		if err != nil {
			return false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
		}

		// no ../ prefix
		// -> does not lie outside of hidden dir
		outsideOfHiddenDir := strings.HasPrefix(relPath, "../")
		isParentDir := relPath == ".."

		if !outsideOfHiddenDir && !isParentDir {
			return true, nil
		}

	}
	return false, nil
}

func AllFiles(fs afero.Fs, dir string) ([]string, error) {
	files := make([]string, 0)

	err := afero.Walk(fs, dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files = append(files, info.Name())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
