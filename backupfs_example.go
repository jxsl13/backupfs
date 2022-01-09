package backupfs

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

	fmt.Println("Overwritten file: ", overwrittenFileContent)
	fmt.Println("Backed up file  : ", backedupContent)

}
