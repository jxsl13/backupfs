package osfs

import (
	"fmt"
	"os/user"

	"golang.org/x/sys/windows"
)

func (f *osFile) OwnerUid() (uid string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get file uid: %w", err)
		}
	}()

	s, err := f.securityDescriptor()
	if err != nil {
		return "", err
	}

	// TODO: what does the _ defaulted value tell us?
	sid, _, err := s.Owner()
	if err != nil {
		return "", err
	}
	defer func() {
		e := windows.FreeSid(sid)
		if err == nil && e != nil {
			err = e
		}
	}()
	return sid.String(), nil
}

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
			err = fmt.Errorf("failed to get file group: %w", err)
		}
	}()

	gid, err := f.OwnerGid()
	if err != nil {
		return "", err
	}

	g, err := user.LookupGroupId(gid)
	if err != nil {
		return "", err
	}
	return g.Name, nil
}

func (f *osFile) OwnerGid() (gid string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get file group: %w", err)
		}
	}()
	s, err := f.securityDescriptor()
	if err != nil {
		return "", err
	}

	// TODO: do we need to check the _ defaulted value?
	sid, _, err := s.Group()
	if err != nil {
		return "", err
	}
	defer func() {
		e := windows.FreeSid(sid)
		if err == nil && e != nil {
			err = e
		}
	}()

	return sid.String(), nil
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

	sid, err := windows.StringToSid(u.Uid)
	if err != nil {
		return err
	}

	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}

	return s.SetOwner(sid, true)
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

	sid, err := windows.StringToSid(g.Gid)
	if err != nil {
		return err
	}

	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}

	return s.SetGroup(sid, true)
}

func (f *osFile) SetOwnerUid(uid string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to set owner uid: %w", err)
		}
	}()

	sid, err := windows.StringToSid(uid)
	if err != nil {
		return err
	}

	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}

	return s.SetOwner(sid, true)
}

func (f *osFile) SetOwnerGid(gid string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to set owner uid: %w", err)
		}
	}()

	sid, err := windows.StringToSid(gid)
	if err != nil {
		return err
	}

	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}

	return s.SetGroup(sid, true)
}

func (f *osFile) securityDescriptor() (sd *windows.SECURITY_DESCRIPTOR, err error) {
	return windows.GetNamedSecurityInfo(f.f.Name(), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
}
