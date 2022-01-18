package internal

import (
	"errors"
	"io/fs"
	"syscall"
)

func Uid(from fs.FileInfo) int {
	return -1
}

func Gid(from fs.FileInfo) int {
	return -1
}

// IgnorableError errors that are due to such functions not being implementedon windows
func IgnorableError(err error) error {
	if errors.Is(err, syscall.EWINDOWS) {
		return nil
	}
	return err
}
