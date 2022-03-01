package internal

import "os"

// EqualMode is os-Dependent
func EqualMode(a, b os.FileMode) bool {
	//
	a &= ChmodBits
	b &= ChmodBits

	return a == b
}

func EqualOwner(a os.FileInfo, uid, gid int) bool {
	return Uid(a) == uid && Gid(a) == gid
}
