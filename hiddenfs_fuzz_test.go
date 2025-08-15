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

		f, err := fs.Create(filePath)
		if err == nil {
			f.Close()
		}

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

		t.Logf("Testing with filePath: %q", filePath)
		fs.RemoveAll(filePath)

		// anything in the hidden directory must stay as is
		t.Logf("Calling countFiles with hiddenDir: %q", hiddenDir)
		countFiles(t, base, hiddenDir, 2)
	})
}
