package internal

import (
	"io/fs"
)

func Uid(from fs.FileInfo) int {
	return -1
}

func Gid(from fs.FileInfo) int {
	return -1
}
