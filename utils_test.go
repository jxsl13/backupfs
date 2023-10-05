package backupfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jxsl13/backupfs/interfaces"
)

func tempDir(ifs interfaces.Fs, dir, prefix string) (name string, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	nconflict := 0
	const maxIterations = 1 << 14
	for i := 0; i < maxIterations; i++ {
		try := filepath.Join(dir, prefix+nextRandom())
		err = ifs.Mkdir(try, 0o700)
		if errors.Is(err, fs.ErrNotExist) {
			if nconflict++; nconflict > 10 {
				randmu.Lock()
				randNum = reseed()
				randmu.Unlock()
			}
			continue
		}
		if err == nil {
			name = try
		}
		break
	}
	return
}

// Random number state.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.
var (
	randNum uint32
	randmu  sync.Mutex
)

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextRandom() string {
	randmu.Lock()
	r := randNum
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	randNum = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}
