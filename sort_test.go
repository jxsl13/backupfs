package backupfs

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByMostFilePathSeparators(t *testing.T) {
	/*
		Edge case where the root path is compared to a file in the root path.
		[0] = "/test/0/2"
		[1] = "/test/0"
		[2] = "/"
		[3] = "/test"
	*/

	table := [][]string{
		{
			"relative_path",
			filepath.Join(separator, "test", "0", "2"),
			separator,
			filepath.Join(separator, "test", "0"),
			filepath.Join(separator, "test"),
		},
	}

	for idx, tt := range table {
		tt := tt
		t.Run(fmt.Sprintf("ByMostFilePathSeparators #%d", idx), func(t *testing.T) {

			sort.Sort(ByMostFilePathSeparators(tt))

			assert.Equal(t, tt[len(tt)-1], separator)

		})
	}
}

func TestByLeastFilePathSeparators(t *testing.T) {
	/*
		Edge case where the root path is compared to a file in the root path.
		[0] = "/test/0/2"
		[1] = "/test/0"
		[2] = "/"
		[3] = "/test"
	*/

	table := [][]string{
		{
			"relative_path",
			filepath.Join(separator, "test", "0", "2"),
			separator,
			filepath.Join(separator, "test", "0"),
			filepath.Join(separator, "test"),
		},
	}

	for idx, tt := range table {
		tt := tt
		t.Run(fmt.Sprintf("ByMostFilePathSeparators #%d", idx), func(t *testing.T) {
			sort.Sort(ByLeastFilePathSeparators(tt))
			assert.Equal(t, tt[0], separator)

		})
	}
}

func FuzzSortByMostFilePathSeparators(f *testing.F) {
	list := []string{
		filepath.Join(separator, "test", "0", "2"),
		filepath.Join(separator, "test", "0"),
		filepath.Join(separator, "test"),
		separator,
	}

	for _, path := range list {
		f.Add(path)
	}

	f.Fuzz(func(t *testing.T, path string) {

		list := []string{
			separator,
			path,
		}
		sort.Sort(ByMostFilePathSeparators(list))

		require.Equal(t, list[len(list)-1], separator)
	})
}

func FuzzSortByLeastFilePathSeparators(f *testing.F) {
	list := []string{
		filepath.Join(separator, "test", "0", "2"),
		filepath.Join(separator, "test", "0"),
		filepath.Join(separator, "test"),
		separator,
	}

	for _, path := range list {
		f.Add(path)
	}

	f.Fuzz(func(t *testing.T, path string) {

		list := []string{
			separator,
			path,
		}
		sort.Sort(ByLeastFilePathSeparators(list))

		require.Equal(t, list[0], separator)
	})
}
