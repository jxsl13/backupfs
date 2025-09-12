package backupfs

import (
	"path/filepath"
)

// TrimVolume trims the volume prefix of a given filepath. C:\A\B\C -> \A\B\C
// highly OS-dependent. On unix systems there is no such thing as a volume path prefix.
func TrimVolume(filePath string) string {
	volume := filepath.VolumeName(filePath)
	return filePath[len(volume):]
}
