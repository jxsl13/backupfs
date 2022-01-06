//go:build go1.18
// +build go1.18

package backupfs

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func NewTestPrefixFs() *PrefixFs {
	prefix, err := filepath.Abs("./tests/prefix")
	if err != nil {
		panic(err)
	}
	return NewPrefixFs(prefix, afero.NewOsFs())
}

func FuzzPrefixFs(f *testing.F) {

	const fileName = "prefixfs_test.txt"
	fs := NewTestPrefixFs()
	for _, seed := range []string{".", "/", "..", "\\", fileName} {
		f.Add(seed)
	}
	// and so on

	expectedPrefix, err := filepath.Abs("./tests/prefix")
	if err != nil {
		f.Fatal(err)
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

func Test_removeStrings(t *testing.T) {
	type args struct {
		lookupIn string
		applyTo  string
		remove   []string
	}
	tests := []struct {
		name        string
		args        args
		wantCleaned string
		wantFound   bool
	}{
		{"#1", args{"..prefixfs_test0txt/../..", "..prefixfs_test0txt/../..", []string{"../", "./", ".."}}, "prefixfs_test0txt/", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCleaned, gotFound := removeStrings(tt.args.lookupIn, tt.args.applyTo, tt.args.remove...)
			if gotCleaned != tt.wantCleaned {
				t.Errorf("removeStrings() gotCleaned = '%v', want '%v'", gotCleaned, tt.wantCleaned)
			}
			if gotFound != tt.wantFound {
				t.Errorf("removeStrings() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}
