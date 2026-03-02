package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestConfig creates a minimal valid lagoon.toml in dir.
func writeTestConfig(t *testing.T, dir string, packages []string, profile string) {
	t.Helper()
	cfg := &config.Config{
		Packages:      packages,
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       profile,
	}
	require.NoError(t, config.Write(filepath.Join(dir, config.Filename), cfg))
}

// TestRunStatusNoConfig verifies that runStatus does not error when lagoon.toml
// is absent — it prints a warning and exits cleanly.
func TestRunStatusNoConfig(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	chdirTemp(t, projectDir) // no lagoon.toml written

	err := runStatus(nil, nil)
	assert.NoError(t, err, "runStatus must return nil when lagoon.toml is missing")
}

// TestRunStatusCacheMiss verifies that runStatus returns nil when config exists
// but no cache has been built yet.
func TestRunStatusCacheMiss(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	writeTestConfig(t, projectDir, []string{"python3"}, "minimal")
	chdirTemp(t, projectDir)

	err := runStatus(nil, nil)
	assert.NoError(t, err, "runStatus must return nil on a cache miss")
}

// TestRunStatusCacheHit verifies that runStatus returns nil when a valid cache
// exists for the current project config.
func TestRunStatusCacheHit(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	cfg := &config.Config{
		Packages:      []string{"python3"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	require.NoError(t, config.Write(filepath.Join(projectDir, config.Filename), cfg))

	// resolve canonical path then build cache using that path
	resolved := chdirTemp(t, projectDir)
	cacheDir := projectCacheDir(resolved)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	sum, err := nix.GenerateShellNix(cfg, shellNixPath)
	require.NoError(t, err)

	fakeEnv := &nix.ResolvedEnv{
		BashPath: "/nix/store/fake-bash/bin/bash",
		EnvPath:  "/nix/store/fake-env/bin/env",
		PATH:     "/nix/store/fake-bash/bin:/nix/store/fake-env/bin",
	}
	require.NoError(t, nix.SaveCache(cacheDir, fakeEnv, sum))

	err = runStatus(nil, nil)
	assert.NoError(t, err, "runStatus must return nil when cache is present and sum matches")
}

// TestRunStatusSumMismatch verifies that runStatus returns nil even when the
// cached sum doesn't match — it's a display command, not a gating check.
func TestRunStatusSumMismatch(t *testing.T) {
	cacheHome := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	cfg := &config.Config{
		Packages:      []string{"python3"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	require.NoError(t, config.Write(filepath.Join(projectDir, config.Filename), cfg))

	resolved := chdirTemp(t, projectDir)
	cacheDir := projectCacheDir(resolved)
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	fakeEnv := &nix.ResolvedEnv{
		BashPath: "/nix/store/fake/bin/bash",
		EnvPath:  "/nix/store/fake/bin/env",
		PATH:     "/nix/store/fake/bin",
	}
	require.NoError(t, nix.SaveCache(cacheDir, fakeEnv, "wrongsum"))

	err := runStatus(nil, nil)
	assert.NoError(t, err, "runStatus must return nil even on sum mismatch")
}
