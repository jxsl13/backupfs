package osfs

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"strconv"
)

func (OsFs) Lchown(name string, username, group string) error {
	uid, gid, err := getIds(username, group)
	if err != nil {
		return &fs.PathError{Op: "lchown", Path: name, Err: err}
	}
	return os.Lchown(name, uid, gid)
}

func (OsFs) Mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(name, perm)
}

func (OsFs) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OsFs) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (OsFs) Chmod(name string, mode fs.FileMode) error {
	// TODO: use custom chmod for Windows: "github.com/hectane/go-acl".Chmod
	// Using the custom implementation implies that we will need to call this chmod everywhere
	// where we pass a fs.FileMode as parameter, e.g. os.OpenFile, os.MkDir, os.MkdirAll, etc.
	return os.Chmod(name, mode)
}

func (OsFs) Chown(name string, username, group string) error {
	uid, gid, err := getIds(username, group)
	if err != nil {
		return &fs.PathError{Op: "chown", Path: name, Err: err}
	}

	return os.Chown(name, uid, gid)
}

func (OsFs) Lstat(name string) (fs.FileInfo, error) {
	return os.Lstat(name)
}

func getIds(username, group string) (uid, gid int, err error) {
	u, err := user.Lookup(username)
	if err != nil {
		return -1, -1, err
	}

	g, err := user.LookupGroup(group)
	if err != nil {
		return -1, -1, err
	}

	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid uid of user %s: %s", username, u.Uid)
	}
	gid, err = strconv.Atoi(g.Gid)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid gid of group %s: %s", group, g.Gid)
	}
	return uid, gid, nil
}
