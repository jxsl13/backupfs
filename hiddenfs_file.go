package backupfs

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/jxsl13/backupfs/internal"
	"github.com/spf13/afero"
)

var _ afero.File = (*HiddenFsFile)(nil)

func newHiddenFsFile(f afero.File, filePath string, hiddenPaths []string) *HiddenFsFile {
	return &HiddenFsFile{
		filePath:    filePath,
		f:           f,
		hiddenPaths: hiddenPaths,
	}
}

type HiddenFsFile struct {
	f           afero.File
	filePath    string
	hiddenPaths []string
}

func (hf *HiddenFsFile) Name() string {
	return hf.f.Name()
}
func (hf *HiddenFsFile) Readdir(count int) ([]os.FileInfo, error) {
	var availableFiles []os.FileInfo
	if count > 0 {
		availableFiles = make([]os.FileInfo, 0, count)
	} else {
		availableFiles = make([]os.FileInfo, 0)
	}

	// extra case where no io.EOF error is returned
	if count <= 0 {
		infos, err := hf.f.Readdir(count)
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			hidden, err := internal.IsHidden(info.Name(), hf.hiddenPaths)
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
			hidden, err := internal.IsHidden(filepath.Join(hf.filePath, info.Name()), hf.hiddenPaths)
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
func (hf *HiddenFsFile) Readdirnames(count int) ([]string, error) {
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
			hidden, err := internal.IsHidden(filepath.Join(hf.filePath, name), hf.hiddenPaths)
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
			hidden, err := internal.IsHidden(name, hf.hiddenPaths)
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
func (hf *HiddenFsFile) Stat() (os.FileInfo, error) {
	return hf.f.Stat()
}
func (hf *HiddenFsFile) Sync() error {
	return hf.f.Sync()
}
func (hf *HiddenFsFile) Truncate(size int64) error {
	return hf.f.Truncate(size)
}
func (hf *HiddenFsFile) WriteString(s string) (ret int, err error) {
	return hf.f.WriteString(s)
}

func (hf *HiddenFsFile) Close() error {
	return hf.f.Close()
}

func (hf *HiddenFsFile) Read(p []byte) (n int, err error) {
	return hf.f.Read(p)
}

func (hf *HiddenFsFile) ReadAt(p []byte, off int64) (n int, err error) {
	return hf.f.ReadAt(p, off)
}

func (hf *HiddenFsFile) Seek(offset int64, whence int) (int64, error) {
	return hf.f.Seek(offset, whence)
}

func (hf *HiddenFsFile) Write(p []byte) (n int, err error) {
	return hf.f.Write(p)
}

func (hf *HiddenFsFile) WriteAt(p []byte, off int64) (n int, err error) {
	return hf.f.WriteAt(p, off)
}
