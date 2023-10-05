package osutils

import (
	"io/fs"
	"syscall"
)

func OwnerUser(path string, from fs.FileInfo) (string, error) {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid)
	}
	// invalid uid = default value
	return -1
}

func OwnerGroup(path string, from fs.FileInfo) (string, error) {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		return int(stat.Gid)
	}
	// invalid uid = default value
	return -1
}
