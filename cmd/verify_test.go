package cmd

import (
	"testing"

	"github.com/imraghavojha/lagoon/internal/nix"
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
