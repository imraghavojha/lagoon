package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
)

func TestProjectCacheDirStable(t *testing.T) {
	got := ProjectCacheDir("/tmp/cache", "/work/project")
	again := ProjectCacheDir("/tmp/cache", "/work/project")
	if got != again {
		t.Fatalf("cache dir changed: %q vs %q", got, again)
	}
	if filepath.Dir(got) != "/tmp/cache" {
		t.Fatalf("cache dir base = %q", got)
	}
}

func TestInspectColdCacheReadOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Packages: []string{"go"}, NixpkgsCommit: config.DefaultCommit, NixpkgsSHA256: config.DefaultSHA256, Profile: "minimal"}
	st := Inspect(cfg, dir, filepath.Join(dir, "cache"))
	if !st.CacheCold || st.CacheReady {
		t.Fatalf("expected cold cache, got %+v", st)
	}
	if _, err := os.Stat(filepath.Join(st.CacheDir, "shell.nix")); !os.IsNotExist(err) {
		t.Fatalf("Inspect should not write shell.nix, stat err=%v", err)
	}
}
