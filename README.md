
# BackupFs

Two filesystem abstraction layers working together to create a straight forward rollback mechanism for filesystem modifications.

Requires the filesystem modifications to happen via the provided structs of this package.

## PrefixFs

This package provides two filesystem abstractions which both implement the spf13/afer.Fs interface as well as the optional interfaces.
Firstly, a struct called `PrefixFs`. As the name already suggests, PrefixFS forces a filesystem to have a specific prefix.
Any attempt to escape the prefix path by directory traversal is prevented, forcing the application to stay within the designated prefix directory.

## BackupFs

The second and more important part of this library is `BackupFs`.
It is a filesystem abstraction that consists of two parts.
A base filesystem and a backup filesystem.
Any attempt to modify a file, directory or symlink in the base filesystem leads to the file being backed up to the backup filesystem.

Consecutive file modifications are ignored, as the initial file state has already been backed up.

## Example

We create a base filesystem with an initial file in it.
Then we define a backup filesystem as subdirectory of the base filesystem.

Then we do wrap the base filesystem and the backup filesystem in the `BackupFs` wrapper and try modifying the file through the `BackupFs` file system layer which has initiall ybeen created in the base filesystem. So `BackupFs` tries to modify an already existing file leading to it being backedup. A call to `BackupFs.Rollback()` allows to rollback the filesystem modifications done with `BackupFs` back to its original state while also deleting the backup.

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jxsl13/backupfs"
	"github.com/spf13/afero"
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
		baseFs   = afero.NewMemMapFs()
		filePath = "/var/opt/test.txt"
	)

	// create an already existing file in base filesystem
	f, err := baseFs.Create(filePath)
	checkErr(err)

	f.WriteString("original text")
	f.Close()

	// at this point we have the base filesystem ready to be ovwerwritten with new files
	var (
		// sub directory in base filesystem as backup directory
		// where the backups should be stored
		backup = backupfs.NewPrefixFs("/var/opt/application/backup", baseFs)

		// backup on write filesystem
		backupFs = backupfs.NewBackupFs(baseFs, backup)
	)

	// we try to override a file in the base filesystem
	f, err = backupFs.Create(filePath)
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
	f.Close()

	backedupContent := string(b)

	f, err = baseFs.Open(filePath)
	checkErr(err)
	b, err = io.ReadAll(f)
	checkErr(err)

	overwrittenFileContent := string(b)

	fmt.Println("Overwritten file: ", overwrittenFileContent)
	fmt.Println("Backed up file  : ", backedupContent)

	afs := afero.Afero{Fs: backupFs}
	fi, err := afs.ReadDir("/var/opt/")
	checkErr(err)

	for _, f := range fi {
		fmt.Println("Found name: ", f.Name())
	}

}
```

