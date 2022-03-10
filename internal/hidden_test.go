package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHidden(t *testing.T) {
	require := require.New(t)
	hiddenDir := "/var/opt/backups"

	rel, hidden, err := IsInHiddenPath("/var/opt", hiddenDir)
	require.NoError(err)
	require.Falsef(hidden, "relpath=%s", rel)

	rel, hidden, err = IsInHiddenPath("/var/opt/test.txt", hiddenDir)
	require.NoError(err)
	require.Falsef(hidden, "relpath=%s", rel)

	rel, hidden, err = IsInHiddenPath("/var/opt/backups/", hiddenDir)
	require.NoError(err)
	require.Truef(hidden, "relpath=%s", rel)

	rel, hidden, err = IsInHiddenPath("/var/opt/backups", hiddenDir)
	require.NoError(err)
	require.Truef(hidden, "relpath=%s", rel)

	rel, hidden, err = IsInHiddenPath("/var/opt/backups/", hiddenDir)
	require.NoError(err)
	require.Truef(hidden, "relpath=%s", rel)

	rel, hidden, err = IsInHiddenPath("/var/opt/backups", hiddenDir)
	require.NoError(err)
	require.Truef(hidden, "relpath=%s", rel)

	rel, hidden, err = IsInHiddenPath("/var/opt/backups/some_file.txt", hiddenDir)
	require.NoError(err)
	require.Truef(hidden, "relpath=%s", rel)

}
