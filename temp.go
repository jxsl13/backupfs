package backupfs

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
)

// TempDir creates a new temporary directory in the directory dir with a name
// that has the prefix prefix and returns the path of the new directory.
// If dir is the empty string, TempDir uses the default OS temp directory.
func TempDir(fsys FS, dir, prefix string) (name string, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	const (
		randLen = 16
		perm    = 0o700
	)

	for i := 0; i < 10000; i++ {
		tmpDirName := randStringFromCharSetWithPrefix(randLen, charSetAlphaNum, prefix)
		try := filepath.Join(dir, tmpDirName)
		err = fsys.MkdirAll(try, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err == nil {
			name = try
		}
		break
	}
	return
}

const (
	// CharSetAlphaNum is the alphanumeric character set for use with
	// randStringFromCharSet
	charSetAlphaNum = "abcdefghijklmnopqrstuvwxyz012346789"
)

// randIntRange returns a random integer between min (inclusive) and max (exclusive)
func randIntRange(min int, max int) int {
	rbi, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		panic(err)
	}

	return min + int(rbi.Int64())
}

// randString generates a random alphanumeric string of the length specified
func randString(strlen int) string {
	return randStringFromCharSet(strlen, charSetAlphaNum)
}

// randStringFromCharSet generates a random string by selecting characters from
// the charset provided
func randStringFromCharSet(strlen int, charSet string) string {
	if strlen <= 0 {
		return ""
	}
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = charSet[randIntRange(0, len(charSet))]
	}
	return string(result)
}

// randStringFromCharSetWithPrefix generates a random string by selecting characters from
// the charset provided
func randStringFromCharSetWithPrefix(strlen int, charSet, prefix string) string {
	return prefix + randStringFromCharSet(strlen, charSet)
}
