package internal

import (
	"io/fs"

	"github.com/spf13/afero"
)

func Chown(from fs.FileInfo, toName string, fs afero.Fs) error {
	return nil
}
