package cmd

import (
	"strings"
	"testing"
)

func TestProjectCacheDirContainsLagoon(t *testing.T) {
	dir := projectCacheDir("/some/project/path")
	if !strings.Contains(dir, "lagoon") {
		t.Errorf("cache dir must contain 'lagoon', got %q", dir)
	}
}

func TestProjectCacheDirIsDeterministic(t *testing.T) {
	a := projectCacheDir("/home/user/myproject")
	b := projectCacheDir("/home/user/myproject")
	if a != b {
		t.Error("same path must produce same cache dir")
	}
}

func TestProjectCacheDirDifferentPaths(t *testing.T) {
	a := projectCacheDir("/home/user/projectA")
	b := projectCacheDir("/home/user/projectB")
	if a == b {
		t.Error("different paths must produce different cache dirs")
	}
}

func TestProjectCacheDirRespectsXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	dir := projectCacheDir("/some/project")
	if !strings.HasPrefix(dir, "/custom/cache") {
		t.Errorf("must use XDG_CACHE_HOME, got %q", dir)
	}
}
