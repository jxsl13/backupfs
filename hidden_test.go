package backupfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHidden(t *testing.T) {
	require := require.New(t)
	hiddenDir := "/var/opt/backups"

	table := []struct {
		path   string
		hidden bool
	}{
		{"/var/opt", false},
		{"/var/opt/test.txt", false},
		{"/var/opt/backups/", true},
		{"/var/opt/backups", true},
		{"/var/opt/backups/", true},
		{"/var/opt/backups/some_file.txt", true},
	}

	for _, row := range table {
		rel, hidden, err := isInHiddenPath(row.path, hiddenDir)
		require.NoError(err)
		require.Equal(row.hidden, hidden, "relpath=%s", rel)
	}
}
