package backupfs

import (
	"testing"
)

// BenchmarkIsInHiddenPath benchmarks the optimized isInHiddenPath function
func BenchmarkHiddenFS_IsInHiddenPath(b *testing.B) {
	// Use absolute paths to test the optimization where filepath.IsAbs returns true
	hiddenDir := "C:\\var\\opt\\backups"
	testPath := "C:\\var\\opt\\backups\\test.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := isInHiddenPath(testPath, hiddenDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIsInHiddenPathRelative benchmarks with relative paths (less optimized case)
func BenchmarkHiddenFS_IsInHiddenPathRelative(b *testing.B) {
	hiddenDir := "/var/opt/backups"
	testPath := "/var/opt/backups/test.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := isInHiddenPath(testPath, hiddenDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDirContains benchmarks the optimized dirContains function
func BenchmarkHiddenFS_DirContains(b *testing.B) {
	parent := "C:\\var\\opt"
	subdir := "C:\\var\\opt\\backups"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dirContains(parent, subdir)
		if err != nil {
			b.Fatal(err)
		}
	}
}
