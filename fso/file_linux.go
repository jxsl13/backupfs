package fso

import (
	"fmt"
	"io/fs"
	"strconv"
	"syscall"
)

func (f *osFile) uid() (uid string, err error) {
	iuid, _, _, err := f.ids()
	if err != nil {
		return "", err
	}

	return strconv.Itoa(iuid), nil
}

func (f *osFile) gid() (gid string, err error) {
	_, igid, _, err := f.ids()
	if err != nil {
		return "", err
	}

	return strconv.Itoa(igid), nil
}

func (f *osFile) own() (uid, gid string, err error) {
	iuid, igid, _, err := f.ids()
	if err != nil {
		return "", "", err
	}
	return strconv.Itoa(iuid), strconv.Itoa(igid), nil
}

func (f *osFile) chown(uid, gid string) (err error) {
	return f.fs.Lchown(f.name(), uid, gid)
}

func (f *osFile) chuid(uid string) (err error) {
	_, cgid, _, err := f.ids()
	if err != nil {
		return err
	}

	return f.fs.Lchown(f.name(), uid, strconv.Itoa(cgid))
}

func (f *osFile) chgid(gid string) (err error) {
	cuid, _, _, err := f.ids()
	if err != nil {
		return err
	}

	return f.fs.Lchown(f.name(), strconv.Itoa(cuid), gid)
}

func (f *osFile) ids() (uid int, gid int, info fs.FileInfo, err error) {
	fi, err := f.fs.Lstat(f.name())
	if err != nil {
		return -1, -1, nil, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, -1, nil, fmt.Errorf("failed to assert *syscall.Stat_t for type: %T", info.Sys())
	}

	return stat.Uid, stat.Gid, fi, nil
}
