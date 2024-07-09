//go:build go1.18
// +build go1.18

package backupfs

import (
	"testing"
)

func FuzzHiddenFSCreate(f *testing.F) {

	for _, seed := range []string{".", "/", "..", "\\", "hidefs_test.txt", "/var/opt/backups", "/var/opt"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, filePath string) {

		_, hiddenDir, _, base, fs := SetupTempDirHiddenFSTest(t)
		// should not be able to create a file in that directory

		fs.Create(filePath)
		fs.MkdirAll(filePath, 0755)
		fs.RemoveAll(filePath)

		// anything in the hidden directory must stay as is
		countFiles(t, base, hiddenDir, 2)
	})
}

func FuzzHiddenFSRemoveAll(f *testing.F) {

	for _, seed := range []string{".", "/", "..", "\\", "hidefs_test.txt", "/var/opt/backups", "/var/opt"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, filePath string) {

		_, hiddenDir, _, base, fs := SetupTempDirHiddenFSTest(t)
		// should not be able to create a file in that directory

		fs.RemoveAll(filePath)

		// anything in the hidden directory must stay as is
		countFiles(t, base, hiddenDir, 2)
	})
}
