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

// ignorableError errors that are due to such functions not being implemented on windows
func ignorableError(err error) error {
	switch {
	case errors.Is(err, syscall.EWINDOWS):
		return nil
	default:
		return err
	}
}
