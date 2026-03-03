package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerRequiresConfig verifies runDocker returns an error when lagoon.toml is absent.
func TestDockerRequiresConfig(t *testing.T) {
	projectDir := t.TempDir()
	chdirTemp(t, projectDir)

	err := runDocker(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lagoon init", "must tell user how to fix it")
}

// TestGenerateDockerNixContainsPackages verifies declared packages appear in docker.nix.
func TestGenerateDockerNixContainsPackages(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Packages:      []string{"python3", "nodejs"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	outPath := filepath.Join(dir, "docker.nix")

	err := nix.GenerateDockerNix(cfg, outPath, "lagoon-testproject")
	require.NoError(t, err)

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "python3")
	assert.Contains(t, string(content), "nodejs")
}

// TestGenerateDockerNixName verifies the image name is substituted into docker.nix.
func TestGenerateDockerNixName(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Packages:      []string{"git"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	outPath := filepath.Join(dir, "docker.nix")

	err := nix.GenerateDockerNix(cfg, outPath, "lagoon-myapp")
	require.NoError(t, err)

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `name = "lagoon-myapp"`,
		"image name must appear in the nix expression")
}

// TestGenerateDockerNixCommit verifies the nixpkgs commit and sha256 appear in docker.nix.
func TestGenerateDockerNixCommit(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Packages:      []string{"curl"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	outPath := filepath.Join(dir, "docker.nix")

	require.NoError(t, nix.GenerateDockerNix(cfg, outPath, "lagoon-x"))

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), config.DefaultCommit, "commit must appear in docker.nix")
	assert.Contains(t, string(content), config.DefaultSHA256, "sha256 must appear in docker.nix")
}

// TestGenerateDockerNixNoPlaceholders verifies no unreplaced {{...}} tokens remain.
func TestGenerateDockerNixNoPlaceholders(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Packages:      []string{"bash"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	outPath := filepath.Join(dir, "docker.nix")

	require.NoError(t, nix.GenerateDockerNix(cfg, outPath, "lagoon-test"))

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.False(t, strings.Contains(string(content), "{{"),
		"docker.nix must have no unreplaced template tokens")
}
