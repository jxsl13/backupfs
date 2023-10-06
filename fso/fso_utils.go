package fso

import (
	"fmt"
	"io"
	"io/fs"
)

// followSymlinks follows all symlinks and returns the filepath of the final file in that chain which is not a symlink) and the file info of that same file
func (ofs *OsFs) followSymlinks(name string) (string, fs.FileInfo, error) {
	visited := make(map[string]struct{}, 2) // need at least 2 symlinks for a cycle
	return ofs.followSymlinksWithVisited(name, visited)
}

// detect cyclic symlinks
func (ofs *OsFs) followSymlinksWithVisited(name string, visited map[string]struct{}) (string, fs.FileInfo, error) {
	if _, found := visited[name]; found {
		return "", nil, fmt.Errorf("symlink cycle found for path: %s", name)
	}

	fi, err := ofs.lstat(name)
	if err != nil {
		return "", nil, err
	}

	if fi.Mode()&fs.ModeSymlink == 0 {
		// not a symlink, simply return name
		return name, fi, nil
	}

	data, err := ofs.readfile(name)
	if err != nil {
		return "", nil, err
	}

	return ofs.followSymlinksWithVisited(string(data), visited)
}

func (ofs *OsFs) readfile(name string) ([]byte, error) {
	f, err := ofs.open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return b, nil
}
