package fsutils

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// ..\ or ../ depending on target os
	RelParent = ".." + string(os.PathSeparator)
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

// DirContains checks if the parent directory contains the subdir
func DirContains(parent, subdir string) (bool, error) {
	relPath, err := filepath.Rel(parent, subdir)
	if err != nil {
		return false, err
	}
	relPath = filepath.FromSlash(relPath)

	isSameDir := relPath == "."
	outsideOfparentDir := strings.HasPrefix(relPath, RelParent) || relPath == ".."

	return !isSameDir && !outsideOfparentDir, nil
}
