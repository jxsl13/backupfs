package internal

import (
	"errors"
	"os"
	"syscall"
)

// reference: os package
var ChmodBits = os.ModePerm

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
