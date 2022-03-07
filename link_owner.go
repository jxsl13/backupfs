package backupfs

// LinkOwner is an optional interface in Afero. It is only implemented by the
// filesystems saying so.
type LinkOwner interface {
	LchownIfPossible(name string, uid int, gid int) error
}
