package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdirTemp changes the working directory to dir and restores it after the
// test. Returns the symlink-resolved path (important on macOS where
// t.TempDir() paths often contain symlinks that os.Getwd() resolves).
func chdirTemp(t *testing.T, dir string) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) })
	// os.Getwd() returns the canonical (symlink-resolved) path after Chdir.
	// this must match what filepath.Abs(".") returns inside runClean/runStatus.
	resolved, err := os.Getwd()
	require.NoError(t, err)
	return resolved
}

// TestRunCleanRemovesCacheDir verifies that runClean deletes the project's
// cache directory when it exists.
func TestRunCleanRemovesCacheDir(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	// resolve after chdir so the hash matches what runClean will compute
	resolved := chdirTemp(t, projectDir)
	cacheDir := projectCacheDir(resolved)
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	require.DirExists(t, cacheDir)

	err := runClean(nil, nil)
	require.NoError(t, err, "runClean must not return an error when cache exists")
	assert.NoDirExists(t, cacheDir, "cache dir must be gone after runClean")
}

// TestRunCleanNoCacheDir verifies that runClean is a no-op (no error) when
// there is no cache for the current project.
func TestRunCleanNoCacheDir(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	chdirTemp(t, projectDir)
	// no cache dir pre-created

	err := runClean(nil, nil)
	assert.NoError(t, err, "runClean must not error when no cache exists")
}

// TestRunCleanDoesNotAffectOtherProjects verifies that cleaning one project's
// cache does not remove the cache for a different project.
func TestRunCleanDoesNotAffectOtherProjects(t *testing.T) {
	cacheHome := t.TempDir()
	projectA := t.TempDir()
	projectB := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	// compute cache dir B before we Chdir (we only clean A)
	// we need B's resolved path too — compute it via a quick chdir
	resolvedB := chdirTemp(t, projectB)
	cacheDirB := projectCacheDir(resolvedB)
	require.NoError(t, os.MkdirAll(cacheDirB, 0755))

	// now set up and clean project A
	resolvedA := chdirTemp(t, projectA)
	cacheDirA := projectCacheDir(resolvedA)
	require.NoError(t, os.MkdirAll(cacheDirA, 0755))

	require.NoError(t, runClean(nil, nil))

	assert.NoDirExists(t, cacheDirA, "project A cache should be removed")
	assert.DirExists(t, cacheDirB, "project B cache must not be touched")
}
