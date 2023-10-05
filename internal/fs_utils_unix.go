//go:build linux || darwin
// +build linux darwin

package internal

import (
	"os"
)

// reference: os package
var ChmodBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

func ignorableChownError(err error) error {
	return err
}

func ignorableChtimesError(err error) error {
	return err
}
