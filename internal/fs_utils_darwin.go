package internal

import (
	"io/fs"
	"syscall"
)

func Uid(from fs.FileInfo) int {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid)
	}
	// invalid uid = default value
	return -1
}

func Gid(from fs.FileInfo) int {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Gid)
	}
	// invalid uid = default value
	return -1
}
