package internal

import (
	"io/fs"
	"syscall"

	"github.com/spf13/afero"
)

// Chown is an operating system dependent implementation.
func Chown(from fs.FileInfo, toName string, fs afero.Fs) error {
	// syscall is OS dependent
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
