package internal

import (
	"io/fs"
	"syscall"

	"github.com/spf13/afero"
)

func Chown(from fs.FileInfo, toName string, fs afero.Fs) error {
	if stat, ok := from.Sys().(*syscall.Stat_t); ok {
		uid := int(stat.Uid)
		gid := int(stat.Gid)

		err := fs.Chown(toName, uid, gid)
		if err != nil {
			return err
		}
	}
	return nil
}
