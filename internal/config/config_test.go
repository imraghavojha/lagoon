package config

import (
	"path/filepath"
	"testing"
)

func TestConfigRoundTripV1Fields(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	cfg := &Config{
		Packages:      []string{"python311", "uv"},
		NixpkgsCommit: DefaultCommit,
		NixpkgsSHA256: DefaultSHA256,
		Profile:       "network",
		Intent:        "dev-workspace",
		Preset:        "python",
		MemoryCap:     "2g",
		Up:            map[string]string{"web": "python3 -m http.server 8000"},
	}
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Intent != cfg.Intent || got.Preset != cfg.Preset || got.MemoryCap != cfg.MemoryCap {
		t.Fatalf("metadata mismatch: %+v", got)
	}
	if got.Up["web"] != cfg.Up["web"] {
		t.Fatalf("up mismatch: %+v", got.Up)
	}
}

func TestReadDefaultsProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	cfg := &Config{Packages: []string{"go"}, NixpkgsCommit: DefaultCommit, NixpkgsSHA256: DefaultSHA256}
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile != "minimal" {
		t.Fatalf("profile = %q", got.Profile)
	}
}
