package backupfs

import (
	"io/fs"
	"strings"
	"time"
)

// filePath and prefix are expected to be normalized (filepath.Clean) paths
func newPrefixFileInfo(base fs.FileInfo, filePath, prefix string) fs.FileInfo {
	var (
		nameOverride = ""
		baseName     = base.Name()
	)

	if filePath == prefix {
		nameOverride = separator
	} else if prefix != "" && strings.HasPrefix(baseName, prefix) {
		nameOverride = strings.TrimPrefix(baseName, prefix)
	}

	return &prefixFileInfo{
		baseFi:       base,
		nameOverride: nameOverride,
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
