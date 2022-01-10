//go:build go1.18
// +build go1.18

package backupfs

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzPrefixFs(f *testing.F) {

	const (
		prefix   = "/some/test/prefix/01/test/02"
		fileName = "prefixfs_test.txt"
	)
	for _, seed := range []string{".", "/", "..", "\\", fileName} {
		f.Add(seed)
	}

	filenameRegex := regexp.MustCompile(`^[^\d]`)

	f.Fuzz(func(t *testing.T, input string) {
		if !filenameRegex.MatchString(input) || len(input) > 256 {
			return
		}
		require := require.New(t)
		fs := NewTestPrefixFs(prefix)

		s, err := fs.prefixPath(input)
		if !strings.HasPrefix(s, prefix) {
			require.Error(err)
			require.True(errors.Is(err, os.ErrNotExist), "expecting returned error to be of type os.ErrNotExist")
			return
		}

		// no error -> we can create a valid file
		require.NoError(err)

		f, err := fs.Create(input)
		require.NoError(err)
		defer func() {
			err := f.Close()
			require.NoError(err)
		}()

		// prefix file must not have any prefix, require that prefix is hidden.
		require.False(strings.HasPrefix(f.Name(), prefix))

	})
}
