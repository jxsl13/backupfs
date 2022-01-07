//go:build go1.18
// +build go1.18

package backupfs

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func FuzzPrefixFs(f *testing.F) {

	expectedPrefix, err := filepath.Abs("./tests/prefix")
	if err != nil {
		f.Fatal(err)
	}

	const fileName = "prefixfs_test.txt"
	fs := NewTestPrefixFs(expectedPrefix)
	for _, seed := range []string{".", "/", "..", "\\", fileName} {
		f.Add(seed)
	}

	filenameRegex := regexp.MustCompile(`[^\d]` + fileName)

	f.Fuzz(func(t *testing.T, input string) {
		if !filenameRegex.MatchString(input) {
			return
		}

		assert := assert.New(t)
		outputPath := fs.prefixPath(input)
		assert.Contains(outputPath, expectedPrefix)
	})
}
