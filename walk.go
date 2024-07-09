package backupfs

import (
	"io/fs"
	"path/filepath"
	"sort"
)

func readDirNames(fs FS, dirname string) ([]string, error) {
	f, err := fs.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func walk(fs FS, path string, info fs.FileInfo, walkFn filepath.WalkFunc) error {
	err := walkFn(path, info, nil)
	if err != nil {
		if info.IsDir() && err == filepath.SkipDir {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return nil
	}

	names, err := readDirNames(fs, path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for _, name := range names {
		filename := filepath.Join(path, name)

		fileInfo, err := fs.Lstat(filename)
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return err
			}
		} else {
			err = walk(fs, filename, fileInfo, walkFn)
			if err != nil {
				if !fileInfo.IsDir() || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}

// Walk walks the file tree rooted at root, calling walkFn for each file
// or directory in the tree, including root. All errors that arise visiting
func Walk(fsys FS, root string, walkFn filepath.WalkFunc) error {
	info, err := fsys.Lstat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walk(fsys, root, info, walkFn)
}
