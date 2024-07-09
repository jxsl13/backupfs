package backupfs

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
)

var _ File = (*HiddenFSFile)(nil)

func newHiddenFSFile(f File, filePath string, hiddenPaths []string) *HiddenFSFile {
	return &HiddenFSFile{
		filePath:    filePath,
		f:           f,
		hiddenPaths: hiddenPaths,
	}
}

type HiddenFSFile struct {
	f           File
	filePath    string
	hiddenPaths []string
}

func (hf *HiddenFSFile) Name() string {
	return hf.f.Name()
}
func (hf *HiddenFSFile) Readdir(count int) ([]fs.FileInfo, error) {
	var availableFiles []fs.FileInfo
	if count > 0 {
		availableFiles = make([]fs.FileInfo, 0, count)
	} else {
		availableFiles = make([]fs.FileInfo, 0)
	}

	// extra case where no io.EOF error is returned
	if count <= 0 {
		infos, err := hf.f.Readdir(count)
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			hidden, err := isHidden(info.Name(), hf.hiddenPaths)
			if err != nil {
				return nil, err
			}
			if !hidden {
				availableFiles = append(availableFiles, info)
			}
		}
		return availableFiles, nil
	}

	for len(availableFiles) < count {
		diff := count - len(availableFiles)
		// diff will become smaller the more often we fetch new file infos
		infos, err := hf.f.Readdir(diff)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		for _, info := range infos {
			hidden, err := isHidden(filepath.Join(hf.filePath, info.Name()), hf.hiddenPaths)
			if err != nil {
				return nil, err
			}
			if !hidden {
				availableFiles = append(availableFiles, info)
			}
		}

		if errors.Is(err, io.EOF) {
			return availableFiles, err
		}
	}

	return availableFiles, nil
}
func (hf *HiddenFSFile) Readdirnames(count int) ([]string, error) {
	var availableFiles []string
	if count > 0 {
		availableFiles = make([]string, 0, count)
	} else {
		availableFiles = make([]string, 0)
	}

	// extra case where no io.EOF error is returned
	if count <= 0 {
		names, err := hf.f.Readdirnames(count)
		if err != nil {
			return nil, err
		}

		for _, name := range names {
			hidden, err := isHidden(filepath.Join(hf.filePath, name), hf.hiddenPaths)
			if err != nil {
				return nil, err
			}
			if !hidden {
				availableFiles = append(availableFiles, name)
			}
		}
		return availableFiles, nil
	}

	for len(availableFiles) < count {
		diff := count - len(availableFiles)

		// diff will become smaller the more often we fetch new file infos
		names, err := hf.f.Readdirnames(diff)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		for _, name := range names {
			hidden, err := isHidden(name, hf.hiddenPaths)
			if err != nil {
				return nil, err
			}
			if !hidden {
				availableFiles = append(availableFiles, name)
			}
		}

		if errors.Is(err, io.EOF) {
			return availableFiles, err
		}
	}

	return availableFiles, nil
}
func (hf *HiddenFSFile) Stat() (fs.FileInfo, error) {
	return hf.f.Stat()
}
func (hf *HiddenFSFile) Sync() error {
	return hf.f.Sync()
}
func (hf *HiddenFSFile) Truncate(size int64) error {
	return hf.f.Truncate(size)
}
func (hf *HiddenFSFile) WriteString(s string) (ret int, err error) {
	return hf.f.WriteString(s)
}

func (hf *HiddenFSFile) Close() error {
	return hf.f.Close()
}

func (hf *HiddenFSFile) Read(p []byte) (n int, err error) {
	return hf.f.Read(p)
}

func (hf *HiddenFSFile) ReadAt(p []byte, off int64) (n int, err error) {
	return hf.f.ReadAt(p, off)
}

func (hf *HiddenFSFile) Seek(offset int64, whence int) (int64, error) {
	return hf.f.Seek(offset, whence)
}

func (hf *HiddenFSFile) Write(p []byte) (n int, err error) {
	return hf.f.Write(p)
}

func (hf *HiddenFSFile) WriteAt(p []byte, off int64) (n int, err error) {
	return hf.f.WriteAt(p, off)
}
