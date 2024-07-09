package backupfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func isParentOfHiddenDir(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(filepath.FromSlash(name))

	for _, hiddenDir := range hiddenPaths {
		isParentOfHiddenDir, err := dirContains(name, hiddenDir)
		if err != nil {
			return false, err
		}
		if isParentOfHiddenDir {
			return true, nil
		}

	}
	return false, nil
}

const relParent = ".." + string(os.PathSeparator)

func dirContains(parent, subdir string) (bool, error) {
	relPath, err := filepath.Rel(parent, subdir)
	if err != nil {
		return false, err
	}
	relPath = filepath.FromSlash(relPath)

	isSameDir := relPath == "."
	outsideOfparentDir := strings.HasPrefix(relPath, relParent) || relPath == ".."

	return !isSameDir && !outsideOfparentDir, nil
}

func isInHiddenPath(name, hiddenDir string) (relPath string, inHiddenPath bool, err error) {
	relPath, err = filepath.Rel(hiddenDir, name)
	if err != nil {
		return "", false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
	}

	relPath = filepath.FromSlash(relPath)

	// no ../ prefix
	// -> does not lie outside of hidden dir
	outsideOfHiddenDir := strings.HasPrefix(relPath, relParent)
	isParentDir := relPath == ".."
	isHiddenDir := relPath == "."

	if !isHiddenDir && (outsideOfHiddenDir || isParentDir) {
		return relPath, false, nil
	}

	return relPath, true, nil
}

// hiddenPaths should be normalized (filepath.Clean result values)
func isHidden(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(filepath.FromSlash(name))

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

func allFiles(fsys FS, dir string) ([]string, error) {
	files := make([]string, 0)

	err := Walk(fsys, dir, func(path string, info fs.FileInfo, err error) error {
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
