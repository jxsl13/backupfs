//go:build linux || darwin
// +build linux darwin

package backupfs

import (
	"syscall"
)

func toSys(uid, gid int) any {
	return &syscall.Stat_t{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}
}
