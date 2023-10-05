package internal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jxsl13/backupfs/fsutils"
)

func IsParentOfHiddenDir(name string, hiddenPaths []string) (bool, error) {
	if len(hiddenPaths) == 0 {
		return false, nil
	}

	// file normalization allows to use a single filepath separator
	name = filepath.Clean(filepath.FromSlash(name))

	for _, hiddenDir := range hiddenPaths {
		isParentOfHiddenDir, err := fsutils.DirContains(name, hiddenDir)
		if err != nil {
			return false, err
		}
		if isParentOfHiddenDir {
			return true, nil
		}

	}
	return false, nil
}

func isInHiddenPath(name, hiddenDir string) (relPath string, inHiddenPath bool, err error) {
	// file normalization allows to use a single filepath separator

	relPath, err = filepath.Rel(hiddenDir, name)
	if err != nil {
		return "", false, &os.PathError{Op: "is_hidden", Path: name, Err: err}
	}

	relPath = filepath.FromSlash(relPath)

	// no ../ prefix
	// -> does not lie outside of hidden dir
	outsideOfHiddenDir := strings.HasPrefix(relPath, fsutils.RelParent)
	isParentDir := relPath == ".."
	isHiddenDir := relPath == "."

	if !isHiddenDir && (outsideOfHiddenDir || isParentDir) {
		return relPath, false, nil
	}

	return relPath, true, nil
}

func IsInHiddenPath(name, hiddenDir string) (relPath string, inHiddenPath bool, err error) {
	// file normalization allows to use a single filepath separator
	name = filepath.Clean(filepath.FromSlash(name))
	return isInHiddenPath(name, hiddenDir)
}

func IsHidden(name string, hiddenPaths []string) (bool, error) {
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
