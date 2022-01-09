package backupfs

import (
	"path/filepath"
)

func iterateDirTree(name string, visitor func(string) error) error {
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
