package mem

import "io/fs"

// FileInfoDirEntry provides an adapter from os.FileInfo to fs.DirEntry
type FileInfoDirEntry struct {
	fs.FileInfo
}

var _ fs.DirEntry = FileInfoDirEntry{}

func (d FileInfoDirEntry) Type() fs.FileMode { return d.FileInfo.Mode().Type() }

func (d FileInfoDirEntry) Info() (fs.FileInfo, error) { return d.FileInfo, nil }
