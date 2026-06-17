package testutils

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRand(t *testing.T) {
	t.Parallel()

	require.GreaterOrEqual(t, RandInt(), 0)

	prefixed := RandIntWithPrefix("name")
	require.True(t, strings.HasPrefix(prefixed, "name-"))

	for i := 0; i < 100; i++ {
		v := RandIntRange(5, 10)
		require.GreaterOrEqual(t, v, 5)
		require.Less(t, v, 10)
	}

	s := RandString(16)
	require.Len(t, s, 16)
	for _, r := range s {
		require.Contains(t, CharSetAlphaNum, string(r))
	}

	s2 := RandStringFromCharSet(8, CharSetAlpha)
	require.Len(t, s2, 8)
	for _, r := range s2 {
		require.Contains(t, CharSetAlpha, string(r))
	}
}

func TestAbsFilePath(t *testing.T) {
	t.Parallel()

	abs := AbsFilePath(t, "relative/path")
	require.True(t, strings.HasSuffix(abs, "path"))
	require.NotEqual(t, "relative/path", abs)
}

func TestFilePath(t *testing.T) {
	t.Parallel()

	p := FilePath("testutils_test.go")
	require.True(t, strings.HasSuffix(p, "testutils_test.go"))

	// build a path that is absolute on the current OS (drive-qualified on windows)
	absPath, err := filepath.Abs("absolute")
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(absPath))
	require.Panics(t, func() {
		FilePath(absPath)
	})
}

func TestFuncIntrospection(t *testing.T) {
	t.Parallel()

	require.Equal(t, "TestFuncIntrospection", FuncName())
	require.Contains(t, FuncSignature(), "TestFuncIntrospection")
	require.Contains(t, FileLine(), "testutils_test.go")

	require.NotEmpty(t, caller(t))
	require.NotEmpty(t, callerLine(t))

	// exercise the explicit up-offset branches
	require.NotEmpty(t, viaOffset(t))
	require.NotEmpty(t, FuncSignature(1))
	require.NotEmpty(t, FileLine(1))
	require.NotEmpty(t, FilePath("testutils_test.go", 1))
	require.NotEmpty(t, CallerFuncName(1))
	require.NotEmpty(t, CallerFileLine(1))
}

// viaOffset calls FuncName with an explicit offset so the up-argument branch is
// covered; offset 1 skips this frame and returns the test's name.
func viaOffset(t *testing.T) string {
	t.Helper()
	return FuncName(1)
}

// helpers introduce an extra stack frame to exercise the Caller* variants.
func caller(t *testing.T) string {
	t.Helper()
	return CallerFuncName()
}

func callerLine(t *testing.T) string {
	t.Helper()
	return CallerFileLine()
}
