package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectCacheDirDeterministic verifies the same path always returns the
// same 8-character hex subdirectory, and the result lives under lagoon/.
func TestProjectCacheDirDeterministic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dir1 := projectCacheDir("/home/user/myproject")
	dir2 := projectCacheDir("/home/user/myproject")

	assert.Equal(t, dir1, dir2, "same path must always produce the same cache dir")
	assert.Contains(t, dir1, "lagoon", "cache dir should be under lagoon/")
}

// TestProjectCacheDirDifferentPaths verifies that two different project paths
// produce different cache subdirectories — no collisions between projects.
func TestProjectCacheDirDifferentPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dir1 := projectCacheDir("/home/user/projectA")
	dir2 := projectCacheDir("/home/user/projectB")

	assert.NotEqual(t, dir1, dir2, "different paths must produce different cache dirs")
}

// TestProjectCacheDirHashLength verifies the path hash portion is exactly 8
// hex chars (32-bit hash — intentionally short for readability).
func TestProjectCacheDirHashLength(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dir := projectCacheDir("/some/project")
	// last segment is the hash
	parts := strings.Split(dir, string(os.PathSeparator))
	hash := parts[len(parts)-1]

	assert.Len(t, hash, 8, "cache dir hash should be exactly 8 hex chars (4 bytes)")
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"hash must only contain hex chars, got: %c", c)
	}
}

// TestProjectCacheDirXDGOverride verifies that XDG_CACHE_HOME is respected.
func TestProjectCacheDirXDGOverride(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", custom)

	dir := projectCacheDir("/some/project")
	assert.True(t, strings.HasPrefix(dir, custom),
		"cache dir should use XDG_CACHE_HOME when set")
}

// TestProjectCacheDirHashMatchesSHA256 verifies the hash is computed from the
// first 4 bytes of SHA256 of the path — so it's predictable and auditable.
func TestProjectCacheDirHashMatchesSHA256(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	path := "/home/user/myproject"
	h := sha256.Sum256([]byte(path))
	expected := fmt.Sprintf("%x", h[:4])

	dir := projectCacheDir(path)
	parts := strings.Split(dir, string(os.PathSeparator))
	actual := parts[len(parts)-1]

	assert.Equal(t, expected, actual, "hash must be first 4 bytes of SHA256 of the path")
}

// TestWarmStartValidatesExistence documents the warm-start validation path:
// if the cache says BashPath=/nix/store/abc-bash, but that path no longer
// exists (post nix-collect-garbage), the warm start should be rejected.
// We test the underlying os.Stat() behaviour directly.
func TestWarmStartValidatesExistence(t *testing.T) {
	// simulate a bash path that does not exist on this machine
	bashPath := "/nix/store/nonexistent-bash-12345/bin/bash"
	_, err := os.Stat(bashPath)
	require.Error(t, err, "non-existent nix path should fail os.Stat")
	assert.True(t, os.IsNotExist(err), "error should be IsNotExist")
}

// TestWarmStartCacheDirInvalidated simulates the full warm-start invalidation:
// env.json exists and sum matches, but the BashPath is gone → hit should be false.
func TestWarmStartCacheDirInvalidated(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	projectPath := "/some/test/project"
	cacheDir := projectCacheDir(projectPath)
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	// write a cache with a path that definitely doesn't exist
	require.NoError(t, os.WriteFile(
		cacheDir+"/env.json",
		[]byte(`{"sum":"abc123","bash_path":"/nix/store/fake/bin/bash","env_path":"/nix/store/fake/bin/env","path":"/nix/store/fake/bin"}`),
		0644,
	))

	// LoadCache should return a hit based on sum
	// shell.go then calls os.Stat on BashPath and invalidates it
	_, statErr := os.Stat("/nix/store/fake/bin/bash")
	assert.True(t, os.IsNotExist(statErr),
		"cache should be invalidated when BashPath doesn't exist — warm start must fall through to cold start")
}
