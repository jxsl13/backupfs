package fso

import (
	"io/fs"
	"os"
	"syscall"

	"github.com/hectane/go-acl"
	"github.com/jxsl13/backupfs/fsi"
	"github.com/jxsl13/backupfs/fsutils"
	"golang.org/x/sys/windows"
)

func (ofs *OsFs) openFile(name string, flag int, perm fs.FileMode) (fsi.File, error) {

	f, err := os.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, err
	}

	// TODO: can we change the file permissions while the file is open?
	if perm != 0 {
		err = ofs.chmod(name, perm)
		if err != nil {
			_ = f.Close()
			return nil, err
		}

	}

	return newOsFile(ofs, f), err
}

func (ofs *OsFs) mkdir(name string, perm fs.FileMode) error {
	err := os.Mkdir(name, perm)
	if err != nil {
		return err
	}
	err = acl.Chmod(name, perm)
	if err != nil {
		return err
	}
	return nil
}

func (ofs *OsFs) mkdirAll(path string, perm fs.FileMode) error {
	dir, err := ofs.stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &fs.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	return fsutils.IterateNotExistingDirTree(ofs, path, func(subdir string, _ fs.FileInfo) error {
		return ofs.mkdir(subdir, perm)
	})
}

func (ofs *OsFs) chmod(name string, mode fs.FileMode) error {
	return acl.Chmod(name, mode)
}

func (ofs *OsFs) chown(name string, uid, gid string) (err error) {
	name, _, err = ofs.followSymlinks(name)
	if err != nil {
		return err
	}

	return ofs.lchown(name, uid, gid)
}

func (ofs *OsFs) lchown(name string, uid, gid string) error {
	return nil
}

func (ofs *OsFs) own(name string) (uid, gid string, err error) {
	name, _, err = ofs.followSymlinks(name)
	if err != nil {
		return "", "", err
	}
	return ofs.lown(name)
}

func (ofs *OsFs) lown(name string) (uid, gid string, err error) {
	s, err := getNamedSecurityDescriptor(name)
	if err != nil {
		return "", "", err
	}

	usid, _, err := s.Owner()
	if err != nil {
		return "", "", err
	}
	defer func() {
		e := windows.FreeSid(usid)
		if e != nil {
			panic(e)
		}
	}()

	gsid, _, err := s.Group()
	if err != nil {
		return "", "", err
	}
	defer func() {
		e := windows.FreeSid(gsid)
		if e != nil {
			panic(e)
		}
	}()

	return usid.String(), gsid.String(), nil
}
