package fsutils_test

import (
	"io/fs"
	"runtime"
	"testing"

	"github.com/jxsl13/backupfs/fso"
	"github.com/jxsl13/backupfs/fsutils"
	"github.com/stretchr/testify/require"
)

func TestIterateNotExistingDirTree(t *testing.T) {
	got := make([]string, 0, 7)

	path := ""
	if runtime.GOOS == "windows" {
		path = "C:\\notExistingDir\\b\\c\\d\\e\\f\\g\\"
	} else {
		path = "/notExistingDir/b/c/d/e/f/g/"
	}

	err := fsutils.IterateNotExistingDirTree(fso.New(), path, func(subdir string, _ fs.FileInfo) error {
		got = append(got, subdir)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 7, len(got))
}

func TestIterateDirTree(t *testing.T) {
	got := make([]string, 0, 8)

	path := ""
	if runtime.GOOS == "windows" {
		path = "C:\\a\\b\\c\\d\\e\\f\\g\\"
	} else {
		path = "/a/b/c/d/e/f/g/"
	}

	err := fsutils.IterateDirTree(fso.New(), path, func(subdir string) error {
		got = append(got, subdir)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 8, len(got))
}
