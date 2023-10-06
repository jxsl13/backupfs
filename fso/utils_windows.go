package fso

import "golang.org/x/sys/windows"

func getNamedSecurityDescriptor(name string) (sd *windows.SECURITY_DESCRIPTOR, err error) {
	return windows.GetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
}
