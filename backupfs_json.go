package backupfs

import (
	"encoding/json"
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

func (fsys *BackupFS) MarshalJSON() ([]byte, error) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	fiMap := make(map[string]*fInfo, len(fsys.baseInfos))

	for path, fi := range fsys.baseInfos {
		if fi == nil {
			fiMap[path] = nil
			continue
		}

		fiMap[path] = toFInfo(path, fi)
	}

	return json.Marshal(fiMap)
}

func (fsys *BackupFS) UnmarshalJSON(data []byte) error {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	fiMap := make(map[string]*fInfo)

	err := json.Unmarshal(data, &fiMap)
	if err != nil {
		return err
	}

	fsys.baseInfos = make(map[string]fs.FileInfo, len(fiMap))
	for k, v := range fiMap {
		if v == nil {
			// required, otherwise the value cannot be checked whethe rit's nil or not
			// due to the additional type information of k, which is of type *fInfo
			fsys.baseInfos[k] = nil
			continue
		}
		fsys.baseInfos[k] = v
	}

	return nil
}
