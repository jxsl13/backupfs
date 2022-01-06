package backupfs

import (
	"path/filepath"
	"strings"
)

var (
	removalSet = []string{"../", "./", "..\\", ".\\"}
)

func cleanPath(name string) string {
	name = strings.TrimRight(name, "./\\")
	found := false
	for {
		slashName := filepath.ToSlash(name)
		name, found = removeStrings(slashName, name, removalSet...)
		if found {
			continue
		}

		break
	}
	return filepath.Clean(name)
}

func removeStrings(lookupIn, applyTo string, remove ...string) (cleaned string, found bool) {
	found = false
	for _, rem := range remove {
		i := -1
		for {
			i++
			idx := -1
			switch i % 2 {
			case 0:
				idx = strings.Index(lookupIn, rem)
			case 1:
				idx = strings.LastIndex(lookupIn, rem)
			}

			// as long as we do find rem, try to replace rem
			if idx >= 0 {
				lookupIn = lookupIn[0:idx] + lookupIn[idx+len(rem):]
				applyTo = applyTo[0:idx] + applyTo[idx+len(rem):]
				found = true
				continue
			}

			break
		}

	}
	return applyTo, found
}

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
