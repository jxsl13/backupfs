package backupfs

import (
	"os"
	"strings"
	"time"
)

func newPrefixFileInfo(base os.FileInfo, prefix string) os.FileInfo {
	return &prefixFileInfo{
		baseFi: base,
		prefix: prefix,
	}
}

// A FileInfo describes a file and is returned by Stat.
type prefixFileInfo struct {
	baseFi os.FileInfo
	prefix string
}

func (fi *prefixFileInfo) Name() string {
	return strings.TrimPrefix(fi.baseFi.Name(), fi.prefix)
}
func (fi *prefixFileInfo) Size() int64 {
	return fi.baseFi.Size()
}
func (fi *prefixFileInfo) Mode() os.FileMode {
	return fi.baseFi.Mode()
}
func (fi *prefixFileInfo) ModTime() time.Time {
	return fi.baseFi.ModTime()
}
func (fi *prefixFileInfo) IsDir() bool {
	return fi.baseFi.IsDir()
}
func (fi *prefixFileInfo) Sys() interface{} {
	return fi.baseFi.Sys()
}
