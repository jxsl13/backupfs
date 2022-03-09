package internal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

func IsParentOfHiddenDir(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(ForceToSlash(name))

	for _, hiddenDir := range hiddenPaths {
		isParentOfHiddenDir, err := DirContains(name, hiddenDir)
		if err != nil {
			return false, err
		}
		if isParentOfHiddenDir {
			return true, nil
		}

	}
	return false, nil
}

func DirContains(parent, subdir string) (bool, error) {
	relPath, err := filepath.Rel(parent, subdir)
	if err != nil {
		return false, err
	}
	relPath = ForceToSlash(relPath)

	isSameDir := relPath == "."
	outsideOfparentDir := strings.HasPrefix(relPath, "../") || relPath == ".."

	return !isSameDir && !outsideOfparentDir, nil
}

func isInHiddenPath(name, hiddenDir string) (relPath string, inHiddenPath bool, err error) {
	// file normalization allows to use a single filepath separator

	relPath, err = filepath.Rel(hiddenDir, name)
	if err != nil {
		return "", false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
	}

	relPath = ForceToSlash(relPath)

	// no ../ prefix
	// -> does not lie outside of hidden dir
	outsideOfHiddenDir := strings.HasPrefix(relPath, "../")
	isParentDir := relPath == ".."
	isHiddenDir := relPath == "."

	if !isHiddenDir && (outsideOfHiddenDir || isParentDir) {
		return relPath, false, nil
	}

	return relPath, true, nil
}

func ForceToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func IsInHiddenPath(name, hiddenDir string) (relPath string, inHiddenPath bool, err error) {
	// file normalization allows to use a single filepath separator
	name = filepath.Clean(ForceToSlash(name))
	return isInHiddenPath(name, hiddenDir)
}

func IsHidden(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(ForceToSlash(name))

	for _, hiddenDir := range hiddenPaths {
		_, hidden, err := isInHiddenPath(name, hiddenDir)
		if err != nil {
			return false, err
		}
		if hidden {
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
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
