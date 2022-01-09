//go:build go1.18
//+build go1.18

package backupfs

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert := assert.New(t)
		fs := NewTestPrefixFs(prefix)

		s, err := fs.prefixPath(input)

		if !strings.HasPrefix(s, prefix) {
			assert.Error(err)
			return
		}

		// no error -> we can create a valid file
		assert.NoError(err)

		f, err := fs.Create(input)
		assert.NoError(err)
		defer func() {
			err := f.Close()
			assert.NoError(err)
		}()

		// prefix file must not have any prefix, assert that prefix is hidden.
		assert.False(strings.HasPrefix(f.Name(), prefix))

	})
}
