//go:build go1.18
// +build go1.18

package backupfs

import (
	"os"
	"strings"
	"testing"

	"github.com/jxsl13/backupfs/internal"
	"github.com/stretchr/testify/require"
)

func FuzzHiddenFsCreate(f *testing.F) {

	for _, seed := range []string{".", "/", "..", "\\", "hidefs_test.txt", "/var/opt/backups", "/var/opt"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, filePath string) {

		_, hiddenDir, _, base, fs := SetupMemMapHiddenFsTest(t)
		if !strings.HasPrefix(filePath, "/var/opt/backups/") {
			return
		}
		// should not be able to create a file in that directory

		require := require.New(t)

		_, err := fs.Create(filePath)
		internal.CountFiles(t, base, hiddenDir, 2)
		require.ErrorIs(err, os.ErrPermission)

	})
}
