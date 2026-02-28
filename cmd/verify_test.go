package cmd

import (
	"testing"
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
