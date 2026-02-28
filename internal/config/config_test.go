package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lagoon.toml")

	cfg := &Config{
		Packages:      []string{"python311", "cowsay"},
		NixpkgsCommit: "abc123",
		NixpkgsSHA256: "sha256abc",
		Profile:       "network",
	}

	if err := Write(path, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(got.Packages) != 2 || got.Packages[0] != "python311" || got.Packages[1] != "cowsay" {
		t.Errorf("packages: got %v", got.Packages)
	}
	if got.NixpkgsCommit != "abc123" {
		t.Errorf("commit: got %q", got.NixpkgsCommit)
	}
	if got.NixpkgsSHA256 != "sha256abc" {
		t.Errorf("sha256: got %q", got.NixpkgsSHA256)
	}
	if got.Profile != "network" {
		t.Errorf("profile: got %q", got.Profile)
	}
}

func TestReadMissing(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadBadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	os.WriteFile(path, []byte("this is not valid toml ]["), 0644)
	_, err := Read(path)
	if err == nil {
		t.Fatal("expected error for invalid toml")
	}
}

func TestDefaultsNonEmpty(t *testing.T) {
	if DefaultCommit == "" {
		t.Error("DefaultCommit must not be empty")
	}
	if DefaultSHA256 == "" {
		t.Error("DefaultSHA256 must not be empty")
	}
	if Filename == "" {
		t.Error("Filename must not be empty")
	}
}
