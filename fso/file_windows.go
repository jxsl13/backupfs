package fso

import (
	"golang.org/x/sys/windows"
)

func (f *osFile) chown(uid, gid string) error {
	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}
	usid, err := windows.StringToSid(uid)
	if err != nil {
		return err
	}
	defer func() {
		e := windows.FreeSid(usid)
		if e != nil {
			panic(e)
		}
	}()

	gsid, err := windows.StringToSid(uid)
	if err != nil {
		return err
	}
	defer func() {
		e := windows.FreeSid(gsid)
		if e != nil {
			panic(e)
		}
	}()
	err = s.SetOwner(usid, true)
	if err != nil {
		return err
	}

	err = s.SetGroup(gsid, true)
	if err != nil {
		return err
	}

	return nil
}

func (f *osFile) own() (uid, gid string, err error) {
	s, err := f.securityDescriptor()
	if err != nil {
		return "", "", err
	}

	// TODO: what does the _ defaulted value tell us?
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

func (f *osFile) uid() (uid string, err error) {
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

func (f *osFile) gid() (gid string, err error) {
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

func (f *osFile) chuid(uid string) (err error) {
	sid, err := windows.StringToSid(uid)
	if err != nil {
		return err
	}
	defer func() {
		windows.FreeSid(sid)
	}()

	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}

	return s.SetOwner(sid, true)
}

func (f *osFile) chgid(gid string) (err error) {

	sid, err := windows.StringToSid(gid)
	if err != nil {
		return err
	}
	defer func() {
		windows.FreeSid(sid)
	}()

	s, err := f.securityDescriptor()
	if err != nil {
		return err
	}

	return s.SetGroup(sid, true)
}

func (f *osFile) securityDescriptor() (sd *windows.SECURITY_DESCRIPTOR, err error) {
	return getNamedSecurityDescriptor(f.name())
}
