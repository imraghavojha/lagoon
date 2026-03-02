package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdirTemp changes the working directory to dir and restores it after the test.
// returns the symlink-resolved path (important on macOS where t.TempDir() paths
// often contain symlinks that os.Getwd() resolves).
func chdirTemp(t *testing.T, dir string) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) })
	resolved, err := os.Getwd()
	require.NoError(t, err)
	return resolved
}

func TestRunRmRemovesCacheDir(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	resolved := chdirTemp(t, projectDir)
	cacheDir := projectCacheDir(resolved)
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	require.DirExists(t, cacheDir)

	err := runRm(nil, nil)
	require.NoError(t, err)
	assert.NoDirExists(t, cacheDir)
}

func TestRunRmNoCacheDir(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	chdirTemp(t, projectDir)

	err := runRm(nil, nil)
	assert.NoError(t, err)
}

func TestRunRmDoesNotAffectOtherProjects(t *testing.T) {
	cacheHome := t.TempDir()
	projectA := t.TempDir()
	projectB := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	resolvedB := chdirTemp(t, projectB)
	cacheDirB := projectCacheDir(resolvedB)
	require.NoError(t, os.MkdirAll(cacheDirB, 0755))

	resolvedA := chdirTemp(t, projectA)
	cacheDirA := projectCacheDir(resolvedA)
	require.NoError(t, os.MkdirAll(cacheDirA, 0755))

	require.NoError(t, runRm(nil, nil))

	assert.NoDirExists(t, cacheDirA)
	assert.DirExists(t, cacheDirB)
}
