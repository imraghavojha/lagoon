package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closure fingerprint helpers (tested here because check.go uses them)

func TestClosureFingerprintIsDeterministic(t *testing.T) {
	paths := []string{"/nix/store/abc-foo", "/nix/store/def-bar", "/nix/store/ghi-baz"}
	assert.Equal(t, closureFingerprint(paths), closureFingerprint(paths))
}

func TestClosureFingerprintIsOrderIndependent(t *testing.T) {
	a := []string{"/nix/store/abc", "/nix/store/def"}
	b := []string{"/nix/store/def", "/nix/store/abc"}
	assert.Equal(t, closureFingerprint(a), closureFingerprint(b))
}

func TestClosureFingerprintChangesWithContent(t *testing.T) {
	assert.NotEqual(t,
		closureFingerprint([]string{"/nix/store/abc-python"}),
		closureFingerprint([]string{"/nix/store/xyz-nodejs"}),
	)
}

func TestClosureFingerprintLength(t *testing.T) {
	assert.Len(t, closureFingerprint([]string{"/nix/store/abc"}), 64, "should be 64-char hex sha256")
}

func TestEnvStorePathsDeduplicates(t *testing.T) {
	env := &nix.ResolvedEnv{
		PATH: "/nix/store/abc-python/bin:/nix/store/def-bash/bin:/nix/store/abc-python/bin",
	}
	assert.Len(t, envStorePaths(env), 2)
}

func TestEnvStorePathsSkipsNonNix(t *testing.T) {
	env := &nix.ResolvedEnv{PATH: "/usr/bin:/nix/store/abc-python/bin"}
	assert.Len(t, envStorePaths(env), 1)
}

func TestEnvStorePathsExtractsParent(t *testing.T) {
	env := &nix.ResolvedEnv{PATH: "/nix/store/abc-python-3.11/bin"}
	paths := envStorePaths(env)
	require.Len(t, paths, 1)
	assert.Equal(t, "/nix/store/abc-python-3.11", paths[0])
}

// baseline file format

func TestCheckBaselineWrittenToFile(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "closure.fingerprint")

	paths := []string{"/nix/store/aaa-bash", "/nix/store/bbb-coreutils"}
	fp := closureFingerprint(paths)
	require.NoError(t, os.WriteFile(baselinePath, []byte(fp), 0644))

	stored, err := os.ReadFile(baselinePath)
	require.NoError(t, err)
	assert.Equal(t, fp, strings.TrimSpace(string(stored)))
}

func TestCheckMatchesBaseline(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "closure.fingerprint")

	paths := []string{"/nix/store/abc-python", "/nix/store/def-bash"}
	baseline := closureFingerprint(paths)
	require.NoError(t, os.WriteFile(baselinePath, []byte(baseline), 0644))

	current := closureFingerprint(paths)
	stored, _ := os.ReadFile(baselinePath)
	assert.Equal(t, strings.TrimSpace(string(stored)), current)
}

func TestCheckDetectsMismatch(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "closure.fingerprint")

	baseline := closureFingerprint([]string{"/nix/store/aaa-bash"})
	require.NoError(t, os.WriteFile(baselinePath, []byte(baseline), 0644))

	current := closureFingerprint([]string{"/nix/store/bbb-cowsay"})
	stored, _ := os.ReadFile(baselinePath)
	assert.NotEqual(t, strings.TrimSpace(string(stored)), current,
		"changed closure must not match stored baseline")
}
