package backupfs

import (
	"io"
	"os"
	"syscall"

	"github.com/spf13/afero"
)

func copyDir(fs afero.Fs, name string, info os.FileInfo) error {
	if !info.IsDir() {
		panic("expecting a directory file-info")
	}

	err := fs.Mkdir(name, info.Mode().Perm())
	if err != nil {
		return err
	}

	err = fs.Chmod(name, info.Mode())
	if err != nil {
		// TODO: do we want to fail here?
		return err
	}

	modTime := info.ModTime()
	err = fs.Chtimes(name, modTime, modTime)
	if err != nil {
		// TODO: do we want to fail here?
		return err
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid := int(stat.Uid)
		gid := int(stat.Gid)

		err = fs.Chown(name, uid, gid)
		if err != nil {
			// TODO: do we want to fail here?
			return err
		}
	}
	return nil
}

func copyFile(fs afero.Fs, name string, info os.FileInfo, sourceFile afero.File) error {
	if info.IsDir() {
		panic("expecting a file file-info")
	}
	file, err := fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}

	_, err = io.Copy(file, sourceFile)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	err = fs.Chmod(name, info.Mode())
	if err != nil {
		return err
	}

	modTime := info.ModTime()
	err = fs.Chtimes(name, modTime, modTime)
	if err != nil {
		return err
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid := int(stat.Uid)
		gid := int(stat.Gid)

		err = fs.Chown(name, uid, gid)
		if err != nil {
			return err
		}
	}
	return nil
}

// Check if a file or directory exists.
func exists(fs afero.Fs, path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
