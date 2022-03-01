package backupfs

// A LinkOwner is a filesystem that is able to change the ownership of symlinks.
type LinkOwner interface {
	LchownIfPossible(path string, uid int, gid int) (bool, error)
}
