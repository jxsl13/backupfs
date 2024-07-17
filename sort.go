package backupfs

import (
	"path/filepath"
	"strings"
)

// current OS filepath separator / or \
const separator = string(filepath.Separator)

// ByMostFilePathSeparators sorts the string by the number of file path separators
// the more nested this is, the further at the beginning of the string slice the path will be
type ByMostFilePathSeparators []string

func (a ByMostFilePathSeparators) Len() int      { return len(a) }
func (a ByMostFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByMostFilePathSeparators) Less(i, j int) bool {
	ca := strings.Count(a[i], separator)
	cb := strings.Count(a[j], separator)

	/*
		Edge case where the root path is compared to a file in the root path.
		[0] = "/test/0/2"
		[1] = "/test/0"
		[2] = "/"
		[3] = "/test"
	*/

	// root = smallest number of separators
	if ca == 1 && a[i] == separator {
		ca = -1
	}

	if cb == 1 && a[j] == separator {
		cb = -1
	}

	if ca == cb {
		return a[i] < a[j]
	}
	return ca > cb
}

// ByLeastFilePathSeparators sorts the string by the number of file path separators
// the least nested the file path is, the further at the beginning it will be of the
// sorted string slice.
type ByLeastFilePathSeparators []string

func (a ByLeastFilePathSeparators) Len() int      { return len(a) }
func (a ByLeastFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByLeastFilePathSeparators) Less(i, j int) bool {
	ca := strings.Count(a[i], separator)
	cb := strings.Count(a[j], separator)

	/*
		Edge case where the root path is compared to a file in the root path.
		[0] = "/test/0/2"
		[1] = "/test/0"
		[2] = "/"
		[3] = "/test"
	*/

	// root = smallest number of separators
	if ca == 1 && a[i] == separator {
		ca = -1
	}

	if cb == 1 && a[j] == separator {
		cb = -1
	}

	if ca == cb {
		return a[i] < a[j]
	}
	return ca < cb
}
