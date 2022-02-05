package internal

import (
	"errors"
	"os"
	"syscall"
)

// reference: os package
var ChmodBits = os.FileMode(0600)

func Uid(from os.FileInfo) int {
	return -1
}

func Gid(from os.FileInfo) int {
	return -1
}

// IgnorableError errors that are due to such functions not being implemented on windows
func IgnorableError(err error) error {
	if errors.Is(err, syscall.EWINDOWS) {
		return nil
	}
	return err
}
