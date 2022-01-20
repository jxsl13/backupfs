package internal

import (
	"errors"
	"os"
	"syscall"
)

func Uid(from os.FileInfo) int {
	return -1
}

func Gid(from os.FileInfo) int {
	return -1
}

// IgnorableError errors that are due to such functions not being implementedon windows
func IgnorableError(err error) error {
	if errors.Is(err, syscall.EWINDOWS) {
		return nil
	}
	return err
}
