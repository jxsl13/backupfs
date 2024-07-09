# BackupFS

Multiple filesystem abstraction layers working together to create a straight forward rollback mechanism for filesystem modifications with OS-independent file paths.
This package provides multiple filesystem abstractions which implement the spf13/afero.FS interface as well as the optional interfaces.

They require the filesystem modifications to happen via the provided structs of this package.

## Example Use Case

My own use case is the ability to implement the command pattern on top of a filesystem.
The pattern consists of a simple interface.

```go
type Command interface {
	Do() error
	Undo() error
}
```

A multitude of such commands allows to provision software packages (archives) and configuration files to target systems running some kind of agent software.
Upon detection of invalid configurations or incorrect software, it is possible to rollback the last transaction.

A transaction is also a command containing a list of non-transaction commands embedding and providing a `BackupFS` to its subcommands requiring to execute filesystem operations.

For all commands solely operating on the filesystem the `Undo()` mechanism consists of simply calling `BackupFS.Rollback()`

Further commands might tackle the topics of:

- un/tar
- creation of files, directories & symlinks
- removal of files, directories & symlinks
- download of files and writing them to the filesystem
- rotation of persisted credentials that might not work upon testing

If you try to tackle the rollback/undo problem yourself you will see pretty fast that the rollback mechanism is a pretty complex implementation with lots of pitfalls where this approach might help you out.

If you follow the rule that **filesystem modifying commands**

- creation,
- deletion
- or modification of files, directories and symlinks
- creation of systemd unit files (writing service configuration)

are to be strictly separated from **side effects causing commands**

- creation of linux system users and groups
- start of linux systemd services configured with the above file in the filesystem

then you will have a much easier time!

## VolumeFS

`VolumeFS` is a filesystem abstraction layer that hides Windows volumes from file system operations.
It allows to define a volume of operation like `c:` or `C:` which is then the only volume that can be accessed.
This abstraction layer allows to operate on filesystems with operating system independent paths.

## PrefixFS

`PrefixFS` forces a filesystem to have a specific prefix.
Any attempt to escape the prefix path by directory traversal is prevented, forcing the application to stay within the designated prefix directory.
This prefix makes the directory basically the application's root directory.

## BackupFS

The most important part of this library is `BackupFS`.
It is a filesystem abstraction that consists of two parts.
A base filesystem and a backup filesystem.
Any attempt to modify a file, directory or symlink in the base filesystem leads to the file being backed up to the backup filesystem.

Consecutive file modifications are ignored, as the initial file state has already been backed up.

## HiddenFS

HiddenFS has a single purpose, that is to hide your backup location and prevent your application from seeing or modifying it.
In case you use BackupFS to backup files that are overwritten on your operating system filesystem (OsFS), you want to define multiple filesystem layers that work together to prevent you from creating a non-terminating recursion of file backups.

- The zero'th layer is the underlying real filesystem, be it the OsFS, MemMapFS, etc.
- The first layer is a VolumeFS filesystem abstraction that removes the need to provide a volume prefix for absolute file paths when accessing files on the underlying filesystem (Windows)
- The second layer is a PrefixFS that is provided a prefix path (backup directory location) and the above instantiated filesystem (e.g. OsFS)
- The third layer is HiddenFS which takes the backup location as path that needs hiding and wraps the first layer in itself.
- The fourth layer is the BackupFS layer which takes the third layer as underlying filesystem to operate on (backup location is not accessible nor viewable) and the second PrefixFS layer to backup your files to.

At the end you will create something along the lines of:

```go
package main

import (
	"os"
	"path/filepath"

	"github.com/jxsl13/backupfs"
)

func main() {

	var (
		// first layer: abstracts away the volume prefix (on Unix the it is an empty string)
		volume     = filepath.VolumeName(os.Args[0]) // determined from application path
		base       = backupfs.NewVolumeFS(volume, backupfs.NewOSFS())
		backupPath = "/var/opt/app/backups"

		// second layer: abstracts away a path prefix
		backup = backupfs.NewPrefixFS(base, backupPath)

		// third layer: hides the backup location in order to prevent recursion
		masked = backupfs.NewHiddenFS(base, backupPath)

		// fourth layer: backup on write filesystem with rollback
		backupFS = backupfs.NewBackupFS(masked, backup)
	)
	// you may use backupFS at this point like the os package
	// except for the backupFS.Rollback() machanism which
	// allows you to rollback filesystem modifications.
}
```

## Example

We create a base filesystem with an initial file in it.
Then we define a backup filesystem as subdirectory of the base filesystem.

Then we do wrap the base filesystem and the backup filesystem in the `BackupFS` wrapper and try modifying the file through the `BackupFS` file system layer which has initiall ybeen created in the base filesystem. So `BackupFS` tries to modify an already existing file leading to it being backedup. A call to `BackupFS.Rollback()` allows to rollback the filesystem modifications done with `BackupFS` back to its original state while also deleting the backup.

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jxsl13/backupfs"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {

	var (
		// base filesystem
		baseFS   = backupfs.NewPrefixFS(backupfs.NewOSFS(), os.TempDir())
		filePath = "/var/opt/test.txt"
	)
	// create an already existing file in base filesystem
	f, err := baseFS.Create(filePath)
	checkErr(err)

	f.WriteString("original text")
	f.Close()

	// at this point we have the base filesystem ready to be overwritten with new files
	var (
		// sub directory in base filesystem as backup directory
		// where the backups should be stored
		backup = backupfs.NewPrefixFS(baseFS, "/var/opt/application/backup")

		// backup on write filesystem
		backupFS = backupfs.NewBackupFS(baseFS, backup)
	)

	// we try to override a file in the base filesystem
	// but in this case we use the backup on write filesystem
	// on top of the base filesystem.
	f, err = backupFS.Create(filePath)
	checkErr(err)
	f.WriteString("new file content")
	f.Close()

	// before we overwrite the file a backup was created
	// at the same path as the overwritten file was found at.
	// due to our backup being on a prefixedfilesystem, we can find
	// the backedup file at a prefixed location

	f, err = backup.Open(filePath)
	checkErr(err)

	b, err := io.ReadAll(f)
	checkErr(err)
	_ = f.Close()

	backedupContent := string(b)

	f, err = baseFS.Open(filePath)
	checkErr(err)
	b, err = io.ReadAll(f)
	checkErr(err)

	overwrittenFileContent := string(b)

	fmt.Println("Overwritten file: ", overwrittenFileContent)
	fmt.Println("Backed up file  : ", backedupContent)

	dir, err := backupFS.Open("/var/opt/")
	checkErr(err)
	defer dir.Close()

	fis, err := dir.Readdir(-1)
	checkErr(err)
	for _, fi := range fis {
		fmt.Println("Found name: ", fi.Name())
	}
}
```

## TODO

- Add symlink fuzz tests on os filesystem that deletes the symlink after each test.
