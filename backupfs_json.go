package backupfs

import (
	"io/fs"
	"path"
	"path/filepath"
	"time"
)

func toFInfo(path string, fi fs.FileInfo) *fInfo {
	return &fInfo{
		FileName:    filepath.ToSlash(path),
		FileMode:    uint32(fi.Mode()),
		FileModTime: fi.ModTime().UnixNano(),
		FileSize:    fi.Size(),
		FileUid:     toUID(fi),
		FileGid:     toGID(fi),
	}
}

type fInfo struct {
	FileName    string `json:"name"`
	FileMode    uint32 `json:"mode"`
	FileModTime int64  `json:"mod_time"`
	FileSize    int64  `json:"size"`
	FileUid     int    `json:"uid"`
	FileGid     int    `json:"gid"`
}

func (fi *fInfo) Name() string {
	return path.Base(fi.FileName)
}
func (fi *fInfo) Size() int64 {
	return fi.FileSize
}
func (fi *fInfo) Mode() fs.FileMode {
	return fs.FileMode(fi.FileMode)
}
func (fi *fInfo) ModTime() time.Time {
	return time.Unix(fi.FileModTime/1000000000, fi.FileModTime%1000000000)
}
func (fi *fInfo) IsDir() bool {
	return fi.Mode().IsDir()
}
func (fi *fInfo) Sys() interface{} {
	return nil
}
