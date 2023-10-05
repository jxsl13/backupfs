package internal

import (
	"os"
)

// EqualMode is os-Dependent
func EqualMode(a, b os.FileMode) bool {
	//
	a &= ChmodBits
	b &= ChmodBits

	return a == b
}
