package cmd

import (
	"os"
	"path/filepath"
	"strings"
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

func TestClosureFingerprintEmptyPaths(t *testing.T) {
	f := closureFingerprint([]string{})
	if len(f) != 64 {
		t.Errorf("empty paths must still produce 64-char fingerprint, got %d chars", len(f))
	}
}

func TestVerifyBaselineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths := []string{"/nix/store/abc-foo", "/nix/store/def-bar"}
	fp := closureFingerprint(paths)

	// write baseline
	baselinePath := filepath.Join(dir, "closure.fingerprint")
	if err := os.WriteFile(baselinePath, []byte(fp), 0644); err != nil {
		t.Fatal(err)
	}

	// read back and verify it matches
	stored, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stored)) != fp {
		t.Error("baseline round-trip mismatch")
	}
}

func TestVerifyBaselineMismatchDetected(t *testing.T) {
	dir := t.TempDir()
	original := closureFingerprint([]string{"/nix/store/abc-python"})
	changed := closureFingerprint([]string{"/nix/store/xyz-nodejs"})

	baselinePath := filepath.Join(dir, "closure.fingerprint")
	os.WriteFile(baselinePath, []byte(original), 0644)

	stored, _ := os.ReadFile(baselinePath)
	if strings.TrimSpace(string(stored)) == changed {
		t.Error("different closures must not match baseline")
	}
}
