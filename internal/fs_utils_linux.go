package internal

import (
	"os"
	"syscall"
)

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

func IgnorableError(err error) error {
	return err
}
