package backupfs

type backupFSOptions struct {
	allowWindowsVolumePaths bool
}

// WithVolumePaths, contrary to the normal BackupFS, this variant allows
// to use absolute windows paths (C:\A\B\C instead of \A\B\C) when set to true.
func WithVolumePaths(allow bool) BackupFSOption {
	return func(bf *backupFSOptions) {
		bf.allowWindowsVolumePaths = allow
	}
}
