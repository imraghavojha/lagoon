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

func TestClosureFingerprintIsDeterministic(t *testing.T) {
	paths := []string{"/nix/store/abc-foo", "/nix/store/def-bar", "/nix/store/ghi-baz"}
	f1 := closureFingerprint(paths)
	f2 := closureFingerprint(paths)
	if f1 != f2 {
		t.Error("fingerprint must be deterministic")
	}
}

func TestClosureFingerprintIsOrderIndependent(t *testing.T) {
	a := []string{"/nix/store/abc", "/nix/store/def"}
	b := []string{"/nix/store/def", "/nix/store/abc"}
	if closureFingerprint(a) != closureFingerprint(b) {
		t.Error("fingerprint must not depend on path order")
	}
}

func TestClosureFingerprintChangesWithContent(t *testing.T) {
	f1 := closureFingerprint([]string{"/nix/store/abc-python"})
	f2 := closureFingerprint([]string{"/nix/store/xyz-nodejs"})
	if f1 == f2 {
		t.Error("different paths must produce different fingerprints")
	}
}

func TestClosureFingerprintLength(t *testing.T) {
	f := closureFingerprint([]string{"/nix/store/abc"})
	if len(f) != 64 {
		t.Errorf("expected 64-char hex sha256, got %d chars", len(f))
	}
}

func TestEnvStorePathsDeduplicates(t *testing.T) {
	env := &nix.ResolvedEnv{
		PATH: "/nix/store/abc-python/bin:/nix/store/def-bash/bin:/nix/store/abc-python/bin",
	}
	paths := envStorePaths(env)
	if len(paths) != 2 {
		t.Errorf("expected 2 unique store paths, got %d: %v", len(paths), paths)
	}
}

func TestEnvStorePathsSkipsNonNix(t *testing.T) {
	env := &nix.ResolvedEnv{
		PATH: "/usr/bin:/nix/store/abc-python/bin",
	}
	paths := envStorePaths(env)
	if len(paths) != 1 {
		t.Errorf("expected 1 nix path, got %d: %v", len(paths), paths)
	}
}

func TestEnvStorePathsExtractsParent(t *testing.T) {
	env := &nix.ResolvedEnv{
		PATH: "/nix/store/abc-python-3.11/bin",
	}
	paths := envStorePaths(env)
	if len(paths) != 1 || paths[0] != "/nix/store/abc-python-3.11" {
		t.Errorf("expected store path without /bin, got %v", paths)
	}
}

// TestVerifyBaselineWrittenToFile verifies the file format of a stored
// baseline — written as a plain hex string, readable back without whitespace.
func TestVerifyBaselineWrittenToFile(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "closure.fingerprint")

	paths := []string{"/nix/store/aaa-bash", "/nix/store/bbb-coreutils"}
	fp := closureFingerprint(paths)

	require.NoError(t, os.WriteFile(baselinePath, []byte(fp), 0644))

	stored, err := os.ReadFile(baselinePath)
	require.NoError(t, err)
	assert.Equal(t, fp, strings.TrimSpace(string(stored)),
		"fingerprint must survive a write/read roundtrip unchanged")
}

// TestVerifyMatchesBaseline verifies that identical path sets produce a
// matching fingerprint when compared to a stored baseline.
func TestVerifyMatchesBaseline(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "closure.fingerprint")

	paths := []string{"/nix/store/abc-python", "/nix/store/def-bash"}
	baseline := closureFingerprint(paths)
	require.NoError(t, os.WriteFile(baselinePath, []byte(baseline), 0644))

	// same paths again
	current := closureFingerprint(paths)
	stored, _ := os.ReadFile(baselinePath)

	assert.Equal(t, strings.TrimSpace(string(stored)), current,
		"same path set must match the stored baseline")
}

// TestVerifyDetectsMismatch verifies that a changed closure (different paths)
// produces a fingerprint that does NOT match the stored baseline.
func TestVerifyDetectsMismatch(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "closure.fingerprint")

	originalPaths := []string{"/nix/store/aaa-bash"}
	changedPaths := []string{"/nix/store/bbb-cowsay"} // different package

	baseline := closureFingerprint(originalPaths)
	require.NoError(t, os.WriteFile(baselinePath, []byte(baseline), 0644))

	current := closureFingerprint(changedPaths)
	stored, _ := os.ReadFile(baselinePath)

	assert.NotEqual(t, strings.TrimSpace(string(stored)), current,
		"changed closure must NOT match the stored baseline — verify should catch this")
}
