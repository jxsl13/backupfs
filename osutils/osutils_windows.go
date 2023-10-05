package osutils

import "io/fs"

func OwnerUser(path string, from fs.FileInfo) (string, error) {
	return "unknown", nil
}

func OwnerGroup(path string, from fs.FileInfo) (string, error) {
	return "unknown", nil
}
