# backupfs & prefixfs

(both filesystem abstractions implement the spf13/afero interfaces)

This package provides two filesystem abstractions.
A structure called `PrefixFs`. As the name already suggests, PrefixFS forces an file system to have a specific prefix.
Any attempt to escape the prefix path by directory traversal should (TM) be prevented, forcing the application to stay within the designated prefix directory.


The second and more important part of this library is `BackupFs`.
It is a filesystem anbstraction that consists of two parts.
A base filesystem and a backup filesystem.
Any attempt to modify a file in the base filesystem leads to the file being backed up to the backup filesystem.

Consecutive file modifications are ignored as the file has already been backed up.

## Fuzzing (beta)

[https://go.dev/blog/fuzz-beta](https://go.dev/blog/fuzz-beta)

Example
```go
import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/afero"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func ExampleBackupFs() {

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
		backup = NewPrefixFs("/var/opt/application/backup", baseFs)

		// backup on write filesystem
		backupFs = NewBackupFs(baseFs, backup)
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

	fmt.Println("Overwritten file content: ", overwrittenFileContent)
	fmt.Println("Backed up file content  : ", backedupContent)

}
```

Small roadmap:

- testing of the backup fs with memory mapped fs
