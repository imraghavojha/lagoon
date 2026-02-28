package nix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContentSumDeterministic(t *testing.T) {
	a := contentSum([]byte("hello lagoon"))
	b := contentSum([]byte("hello lagoon"))
	if a != b {
		t.Errorf("same input gave different sums: %q vs %q", a, b)
	}
}

func TestContentSumChanges(t *testing.T) {
	a := contentSum([]byte("aaa"))
	b := contentSum([]byte("bbb"))
	if a == b {
		t.Error("different inputs gave same sum")
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	dir := t.TempDir()
	env := &ResolvedEnv{
		BashPath: "/nix/store/abc/bin/bash",
		EnvPath:  "/nix/store/def/bin/env",
		PATH:     "/nix/store/abc/bin:/nix/store/def/bin",
	}
	sum := "deadbeef"

	if err := SaveCache(dir, env, sum); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	// verify file was created
	if _, err := os.Stat(filepath.Join(dir, "env.json")); err != nil {
		t.Fatalf("env.json not created: %v", err)
	}

	got, hit := LoadCache(dir, sum)
	if !hit {
		t.Fatal("expected cache hit")
	}
	if got.BashPath != env.BashPath {
		t.Errorf("BashPath: got %q want %q", got.BashPath, env.BashPath)
	}
	if got.PATH != env.PATH {
		t.Errorf("PATH: got %q want %q", got.PATH, env.PATH)
	}
}

func TestLoadCacheWrongSum(t *testing.T) {
	dir := t.TempDir()
	env := &ResolvedEnv{BashPath: "/nix/store/abc/bin/bash", EnvPath: "/nix/store/def/bin/env"}
	SaveCache(dir, env, "sum-a")

	_, hit := LoadCache(dir, "sum-b")
	if hit {
		t.Error("expected cache miss for wrong sum")
	}
}

func TestLoadCacheMissingFile(t *testing.T) {
	_, hit := LoadCache(t.TempDir(), "anysum")
	if hit {
		t.Error("expected cache miss for empty dir")
	}
}

func TestLoadCacheCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "env.json"), []byte("not json}{"), 0644)
	_, hit := LoadCache(dir, "anysum")
	if hit {
		t.Error("expected cache miss for corrupt JSON")
	}
}
