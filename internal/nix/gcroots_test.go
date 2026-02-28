package nix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateGCRootsCreatesSymlinks(t *testing.T) {
	dir := t.TempDir()
	env := &ResolvedEnv{
		PATH: "/nix/store/abc-python-3.11/bin:/nix/store/def-bash-5.2/bin",
	}
	CreateGCRoots(dir, env)

	gcDir := filepath.Join(dir, "gcroots")
	entries, err := os.ReadDir(gcDir)
	if err != nil {
		t.Fatalf("gcroots dir not created: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 GC root symlinks, got %d", len(entries))
	}
	// each entry should be a symlink
	for _, e := range entries {
		info, err := os.Lstat(filepath.Join(gcDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected symlink, got %v", info.Mode())
		}
	}
}

func TestCreateGCRootsDeduplicates(t *testing.T) {
	dir := t.TempDir()
	// same store path twice — should only create one symlink
	env := &ResolvedEnv{
		PATH: "/nix/store/abc-pkg/bin:/nix/store/abc-pkg/bin",
	}
	CreateGCRoots(dir, env)

	entries, _ := os.ReadDir(filepath.Join(dir, "gcroots"))
	if len(entries) != 1 {
		t.Errorf("expected 1 symlink for duplicate paths, got %d", len(entries))
	}
}

func TestCreateGCRootsRestoresDeletedSymlink(t *testing.T) {
	dir := t.TempDir()
	env := &ResolvedEnv{PATH: "/nix/store/abc-python/bin"}

	CreateGCRoots(dir, env)
	gcDir := filepath.Join(dir, "gcroots")
	entries, _ := os.ReadDir(gcDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 symlink after first call, got %d", len(entries))
	}

	// delete the symlink
	os.Remove(filepath.Join(gcDir, entries[0].Name()))

	// second call must restore it
	CreateGCRoots(dir, env)
	entries, _ = os.ReadDir(gcDir)
	if len(entries) != 1 {
		t.Errorf("expected symlink to be restored, got %d entries", len(entries))
	}
}

func TestCreateGCRootsSkipsNonNixPaths(t *testing.T) {
	dir := t.TempDir()
	env := &ResolvedEnv{
		PATH: "/usr/bin:/usr/local/bin",
	}
	CreateGCRoots(dir, env)

	gcDir := filepath.Join(dir, "gcroots")
	entries, _ := os.ReadDir(gcDir)
	if len(entries) != 0 {
		t.Errorf("non-nix paths must not create GC roots, got %d entries", len(entries))
	}
}
