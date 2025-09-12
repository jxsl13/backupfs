package testutils

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func AbsFilePath(t *testing.T, p string) string {
	p, err := filepath.Abs(p)
	require.NoError(t, err)
	return p
}
