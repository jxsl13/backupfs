package backupfs

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzPrefixFS(f *testing.F) {

	var (
		rootPath = CallerPathTmp()
		rootFS   = NewTempDirPrefixFS(rootPath)
		prefix   = filepath.FromSlash("/some/test/prefix/01/test/02")
		fileName = "prefixfs_test.txt"
	)
	fsys, err := NewPrefixFS(rootFS, prefix)
	if err != nil {
		f.Fatal(err)
	}

	prefix, err = filepath.Abs(prefix)
	if err != nil {
		f.Fatal(err)
	}

	for _, seed := range []string{".", "/", "..", "\\", fileName} {
		f.Add(seed)
	}

	filenameRegex := regexp.MustCompile(`^[^\d]`)

	f.Fuzz(func(t *testing.T, input string) {
		if !filenameRegex.MatchString(input) || len(input) > 256 {
			return
		}
		require := require.New(t)

		_, s, err := fsys.prefixPath(input)
		if err != nil {
			// ignore returned errors
			return
		}

		// if we were able to prefix the path then the prefix must be present
		if !strings.HasPrefix(s, fsys.prefix) {
			require.Error(err)
			require.ErrorIs(err, fs.ErrNotExist, "expecting returned error to be of type fs.ErrNotExist")
			return
		}

		// prefix file must not have any prefix, require that prefix is hidden.
		hasPrefix := strings.HasPrefix(f.Name(), fsys.prefix)
		require.Falsef(hasPrefix, "expecting file to not have prefix: %v", prefix)

	})
}
