package fsutils

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jxsl13/backupfs/fsi"
)

const (
	// ..\ or ../ depending on target os
	RelParent = ".." + string(os.PathSeparator)
)

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

func IterateDirTree(f fsi.Fs, path string, visitor func(subdir string) error) (err error) {

	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent.
		err = IterateDirTree(f, path[:j-1], visitor)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke visitor and use its result.
	err = visitor(path)
	if err != nil {
		return err
	}
	return nil
}

func IterateNotExistingDirTree(f fsi.Fs, path string, visitor func(subdir string, fi fs.FileInfo) error) (err error) {

	dir, err := f.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &fs.PathError{Op: "stat", Path: path, Err: syscall.ENOTDIR}
	}

	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent.
		err = IterateNotExistingDirTree(f, path[:j-1], visitor)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke visitor and use its result.
	err = visitor(path, dir)
	if err != nil {
		return err
	}
	return nil
}
