package backupfs

import (
	"io/fs"
	"path/filepath"
	"time"
)

// filePath and prefix are expected to be normalized (filepath.Clean) paths
func newPrefixFileInfo(base fs.FileInfo, absolute string) fs.FileInfo {
	return &prefixFileInfo{
		baseFi:       base,
		nameOverride: filepath.Base(absolute),
	}
}

// A FileInfo describes a file and is returned by Stat.
type prefixFileInfo struct {
	baseFi       fs.FileInfo
	nameOverride string
}

func (fi *prefixFileInfo) Name() string {
	if fi.nameOverride != "" {
		return fi.nameOverride
	}
	return fi.baseFi.Name()
}
func (fi *prefixFileInfo) Size() int64 {
	return fi.baseFi.Size()
}
func (fi *prefixFileInfo) Mode() fs.FileMode {
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
