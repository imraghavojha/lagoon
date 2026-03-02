package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeToml(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "lagoon.toml")
	os.WriteFile(f, []byte(content), 0644)
	return f
}

func TestReadValidConfig(t *testing.T) {
	f := writeToml(t, `
packages = ["cowsay"]
nixpkgs_commit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "minimal"
`)
	if _, err := Read(f); err != nil {
		t.Fatalf("valid config must parse without error: %v", err)
	}
}

func TestReadBadCommitLength(t *testing.T) {
	f := writeToml(t, `
packages = ["cowsay"]
nixpkgs_commit = "short"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "minimal"
`)
	_, err := Read(f)
	if err == nil {
		t.Error("expected error for commit with wrong length")
	}
}

func TestReadBadSHA256Length(t *testing.T) {
	f := writeToml(t, `
packages = ["cowsay"]
nixpkgs_commit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
nixpkgs_sha256 = "tooshort"
profile = "minimal"
`)
	_, err := Read(f)
	if err == nil {
		t.Error("expected error for sha256 with wrong length")
	}
}
