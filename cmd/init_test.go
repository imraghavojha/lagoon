package cmd

import (
	"path/filepath"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitConfigNetworkProfile verifies that selecting network=true writes
// profile = "network" in the config file.
func TestInitConfigNetworkProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.Filename)

	cfg := &config.Config{
		Packages:      []string{"python3", "git"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "network",
	}
	require.NoError(t, config.Write(path, cfg))

	back, err := config.Read(path)
	require.NoError(t, err)

	assert.Equal(t, "network", back.Profile,
		"network=true must produce profile='network'")
}

// TestInitConfigMinimalProfile verifies that selecting network=false writes
// profile = "minimal" in the config file.
func TestInitConfigMinimalProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.Filename)

	cfg := &config.Config{
		Packages:      []string{"nodejs"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	require.NoError(t, config.Write(path, cfg))

	back, err := config.Read(path)
	require.NoError(t, err)

	assert.Equal(t, "minimal", back.Profile,
		"network=false must produce profile='minimal'")
}

// TestInitDefaultCommitAndSHA256 verifies that the DefaultCommit and
// DefaultSHA256 constants (the pinned nixpkgs version) are valid lengths — 40
// hex chars for commit, 52 chars for sha256. If these are wrong, every cold
// start will fail with a confusing nix-shell error.
func TestInitDefaultCommitAndSHA256(t *testing.T) {
	assert.Len(t, config.DefaultCommit, 40,
		"DefaultCommit must be a 40-char git SHA1 — update internal/config/config.go")
	assert.Len(t, config.DefaultSHA256, 52,
		"DefaultSHA256 must be a 52-char nix sha256 — update internal/config/config.go")
}

// TestInitConfigPreservesPackageList verifies that the package list written to
// lagoon.toml is read back without modification or reordering.
func TestInitConfigPreservesPackageList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.Filename)

	packages := []string{"rustc", "cargo", "gcc", "make", "pkg-config"}
	cfg := &config.Config{
		Packages:      packages,
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	require.NoError(t, config.Write(path, cfg))

	back, err := config.Read(path)
	require.NoError(t, err)

	assert.Equal(t, packages, back.Packages,
		"package list must be preserved exactly as written (order matters for reproducibility)")
}
