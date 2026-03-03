package cmd

import (
	"testing"

	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvStorePathsDeduplicates(t *testing.T) {
	env := &nix.ResolvedEnv{
		PATH: "/nix/store/abc-python/bin:/nix/store/def-bash/bin:/nix/store/abc-python/bin",
	}
	assert.Len(t, envStorePaths(env), 2)
}

func TestEnvStorePathsSkipsNonNix(t *testing.T) {
	env := &nix.ResolvedEnv{PATH: "/usr/bin:/nix/store/abc-python/bin"}
	assert.Len(t, envStorePaths(env), 1)
}

func TestEnvStorePathsExtractsParent(t *testing.T) {
	env := &nix.ResolvedEnv{PATH: "/nix/store/abc-python-3.11/bin"}
	paths := envStorePaths(env)
	require.Len(t, paths, 1)
	assert.Equal(t, "/nix/store/abc-python-3.11", paths[0])
}
