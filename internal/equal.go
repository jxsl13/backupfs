package internal

import "io/fs"

// EqualMode is os-Dependent
func EqualMode(a, b fs.FileMode) bool {
	//
	a &= ChmodBits
	b &= ChmodBits

	return a == b
}
