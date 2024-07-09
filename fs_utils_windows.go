package backupfs

import (
	"errors"
	"io/fs"
	"syscall"
)

// reference: os package
var chmodBits fs.FileMode = 0600

func Uid(from fs.FileInfo) int {
	return -1
}

func Gid(from fs.FileInfo) int {
	return -1
}

// ignorableError errors that are due to such functions not being implemented on windows
func ignorableChownError(err error) error {
	switch {
	case errors.Is(err, syscall.EWINDOWS):
		return nil
	default:
		return err
	}
}

func ignorableChtimesError(err error) error {
	return err
}
