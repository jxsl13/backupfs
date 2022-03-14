//go:build linux || darwin
// +build linux darwin

package internal

import (
	"os"
	"syscall"
)

// reference: os package
var ChmodBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

func Uid(from os.FileInfo) int {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid)
	}
	// invalid uid = default value
	return -1
}

func Gid(from os.FileInfo) int {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Gid)
	}
	// invalid uid = default value
	return -1
}

func ignorableChownError(err error) error {
	return err
}

func ignorableChtimesError(err error) error {
	return err
}
