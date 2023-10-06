package fso

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"syscall"

	"github.com/jxsl13/backupfs/fsi"
)

func (ofs *OsFs) openFile(name string, flag int, perm fs.FileMode) (fsi.File, error) {
	f, e := os.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return newOsFile(ofs, f), e
}

func (ofs *OsFs) mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(name, perm)
}

func (ofs *OsFs) mkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (ofs *OsFs) chmod(name string, mode fs.FileMode) error {
	return os.Chmod(name, mode)
}

func (ofs *OsFs) lchown(name string, uid, gid string) error {
	iuid, igid, err := parseIds(uid, gid)
	if err != nil {
		return err
	}
	return os.Lchown(name, iuid, igid)
}

func (ofs *OsFs) lown(name string) (uid, gid string, err error) {

	fi, err := ofs.lstat(name)
	if err != nil {
		return "", "", &fs.PathError{Op: "lown", Path: name, Err: err}
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", &fs.PathError{Op: "lown", Path: name, Err: fmt.Errorf("failed to assert syscall.Stat_t: %T", fi.Sys())}
	}

	return strconv.Itoa(stat.Uid), strconv.Itoa(stat.Gid), nil
}

func (ofs *OsFs) own(name string) (uid, gid string, err error) {
	fi, err := ofs.lstat(name)
	if err != nil {
		return "", "", &fs.PathError{Op: "lown", Path: name, Err: err}
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", &fs.PathError{Op: "lown", Path: name, Err: fmt.Errorf("failed to assert syscall.Stat_t: %T", fi.Sys())}
	}

	return strconv.Itoa(stat.Uid), strconv.Itoa(stat.Gid), nil
}

func (ofs *OsFs) chown(name string, uid, gid string) error {
	iuid, igid, err := parseIds(uid, gid)
	if err != nil {
		return &fs.PathError{Op: "chown", Path: name, Err: err}
	}

	return os.Chown(name, iuid, igid)
}

func parseIds(suid, sgid string) (uid, gid int, err error) {
	uid, err = strconv.Atoi(suid)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid uid: %s", suid)
	}

	gid, err = strconv.Atoi(suid)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid gid: %s", sgid)
	}

	return uid, gid, nil
}
