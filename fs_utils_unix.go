//go:build linux || darwin
// +build linux darwin

package backupfs

import (
	"io/fs"
	"syscall"
)

// reference: os package
var chmodBits fs.FileMode = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

func toUID(from fs.FileInfo) int {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid)
	}
	// invalid uid = default value
	return -1
}

func toGID(from fs.FileInfo) int {
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
