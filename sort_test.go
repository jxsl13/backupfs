package backupfs

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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
		
		// Store original list for comparison
		originalSeparatorSeparators := strings.Count(TrimVolume(separator), separator)
		originalPathSeparators := strings.Count(TrimVolume(path), separator)
		
		// Apply special root handling like LessFilePathSeparators does
		trimmedSeparator := TrimVolume(separator)
		if originalSeparatorSeparators == 1 && trimmedSeparator == separator {
			originalSeparatorSeparators = -1
		}
		
		trimmedPath := TrimVolume(path)
		if originalPathSeparators == 1 && trimmedPath == separator {
			originalPathSeparators = -1
		}
		
		sort.Sort(ByMostFilePathSeparators(list))
		
		// After sorting by most separators, the element with fewer separators should be last
		// If they have the same number of separators, lexicographic order determines position
		if originalSeparatorSeparators < originalPathSeparators {
			require.Equal(t, list[len(list)-1], separator)
		} else if originalSeparatorSeparators > originalPathSeparators {
			require.Equal(t, list[0], separator)
		} else {
			// Same number of separators, check the actual sort result using the Less function
			// If ByMostFilePathSeparators.Less(separator, path) is true, then separator comes before path
			byMost := ByMostFilePathSeparators([]string{separator, path})
			if byMost.Less(0, 1) { // separator < path in ByMostFilePathSeparators order
				require.Equal(t, list[0], separator)
				require.Equal(t, list[1], path)
			} else {
				require.Equal(t, list[0], path)
				require.Equal(t, list[1], separator)
			}
		}
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
		
		// Store original list for comparison
		originalSeparatorSeparators := strings.Count(TrimVolume(separator), separator)
		originalPathSeparators := strings.Count(TrimVolume(path), separator)
		
		// Apply special root handling like LessFilePathSeparators does
		trimmedSeparator := TrimVolume(separator)
		if originalSeparatorSeparators == 1 && trimmedSeparator == separator {
			originalSeparatorSeparators = -1
		}
		
		trimmedPath := TrimVolume(path)
		if originalPathSeparators == 1 && trimmedPath == separator {
			originalPathSeparators = -1
		}
		
		sort.Sort(ByLeastFilePathSeparators(list))
		
		// After sorting by least separators, the element with fewer separators should be first
		if originalSeparatorSeparators < originalPathSeparators {
			require.Equal(t, list[0], separator)
		} else if originalSeparatorSeparators > originalPathSeparators {
			require.Equal(t, list[1], separator)
		} else {
			// Same number of separators, check the actual sort result using the Less function
			// If ByLeastFilePathSeparators.Less(separator, path) is true, then separator comes before path
			byLeast := ByLeastFilePathSeparators([]string{separator, path})
			if byLeast.Less(0, 1) { // separator < path in ByLeastFilePathSeparators order
				require.Equal(t, list[0], separator)
				require.Equal(t, list[1], path)
			} else {
				require.Equal(t, list[0], path)
				require.Equal(t, list[1], separator)
			}
		}
	})
}
