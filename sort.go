package backupfs

import (
	"path/filepath"
	"strings"
)

// current OS filepath separator / or \
const separator = string(filepath.Separator)

// byMostFilePathSeparators sorts the string by the number of file path separators
// the more nested this is, the further at the beginning of the string slice the path will be
type byMostFilePathSeparators []string

func (a byMostFilePathSeparators) Len() int      { return len(a) }
func (a byMostFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byMostFilePathSeparators) Less(i, j int) bool {

	return strings.Count(a[i], separator) > strings.Count(a[j], separator)
}

// byLeastFilePathSeparators sorts the string by the number of file path separators
// the least nested the file path is, the further at the beginning it will be of the
// sorted string slice.
type byLeastFilePathSeparators []string

func (a byLeastFilePathSeparators) Len() int      { return len(a) }
func (a byLeastFilePathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byLeastFilePathSeparators) Less(i, j int) bool {

	return strings.Count(a[i], separator) < strings.Count(a[j], separator)
}
