package osfs

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func (f *osFile) OwnerUser() (owner string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get file owner: %w", err)
		}
	}()

	uid, err := f.OwnerUid()
	if err != nil {
		return "", err
	}
	u, err := user.LookupId(uid)
	if err != nil {
		return "", err
	}
	return u.Username, nil

}
func (f *osFile) OwnerGroup() (group string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get file owner: %w", err)
		}
	}()

	gid, err := f.OwnerGroup()
	if err != nil {
		return "", err
	}
	g, err := user.LookupGroupId(gid)
	if err != nil {
		return "", err
	}
	return g.Name, nil
}

func (f *osFile) OwnerUid() (string, error) {
	uid, err := f.uid()
	if err != nil {
		return "", fmt.Errorf("failed to get owner uid: %w", err)
	}
	return strconv.Itoa(uid), nil
}

func (f *osFile) OwnerGid() (string, error) {
	gid, err := f.gid()
	if err != nil {
		return "", fmt.Errorf("failed to get owner gid: %w", err)
	}
	return strconv.Itoa(gid), nil
}

func (f *osFile) SetOwnerUser(username string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to set owner user: %w", err)
		}
	}()

	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	return f.SetOwnerUid(u.Uid)
}

func (f *osFile) SetOwnerGroup(group string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to set owner group: %w", err)
		}
	}()

	g, err := user.LookupGroup(group)
	if err != nil {
		return err
	}
	return f.SetOwnerGid(g.Gid)
}

func (f *osFile) SetOwnerUid(newUid string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to set owner uid: %w", err)
		}
	}()

	iuid, err := strconv.Atoi(newUid)
	if err != nil {
		return fmt.Errorf("invalid uid: expected integer value: %q: %w", newUid, err)
	}

	uid, gid, fi, err := f.ids()
	if err != nil {
		return err
	}

	if iuid == uid {
		// old and new are the same
		// nothing to do
		return nil
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Lchown(f.f.Name(), iuid, gid)
	}

	return os.Chown(f.f.Name(), iuid, gid)
}

func (f *osFile) SetOwnerGid(newGid string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to set owner uid: %w", err)
		}
	}()

	igid, err := strconv.Atoi(newGid)
	if err != nil {
		return fmt.Errorf("invalid gid: expected integer value: %q: %w", newGid, err)
	}

	uid, gid, fi, err := f.ids()
	if err != nil {
		return err
	}

	if igid == gid {
		// old and new are the same
		// nothing to do
		return nil
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Lchown(f.f.Name(), uid, igid)
	}

	return os.Chown(f.f.Name(), uid, igid)
}

func uidOf(info fs.FileInfo) (int, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid), nil
	}
	return -1, fmt.Errorf("failed to get uid of file: %s", info.Name())
}

func gidOf(info fs.FileInfo) (int, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(stat.Gid), nil
	}
	return -1, fmt.Errorf("failed to get gid of file: %s", info.Name())
}

func (f *osFile) uid() (int, error) {
	fi, err := f.f.Stat()
	if err != nil {
		return -1, err
	}

	uid, err := uidOf(fi)
	if err != nil {
		return -1, err
	}
	return uid, nil
}

func (f *osFile) gid() (int, error) {
	fi, err := f.f.Stat()
	if err != nil {
		return -1, err
	}

	gid, err := gidOf(fi)
	if err != nil {
		return -1, err
	}
	return gid, nil
}

func (f *osFile) ids() (uid int, gid int, info fs.FileInfo, err error) {
	fi, err := f.f.Stat()
	if err != nil {
		return -1, -1, nil, err
	}

	uid, err = uidOf(fi)
	if err != nil {
		return -1, -1, nil, err
	}

	gid, err = gidOf(fi)
	if err != nil {
		return -1, -1, nil, err
	}

	return uid, gid, fi, nil
}
