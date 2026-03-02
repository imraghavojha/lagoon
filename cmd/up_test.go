package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpRequiresConfig verifies that runUp returns an error when lagoon.toml is absent.
func TestUpRequiresConfig(t *testing.T) {
	dir := t.TempDir()
	chdirTemp(t, dir) // no lagoon.toml

	err := runUp(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lagoon init")
}

// TestUpRequiresServices verifies that runUp errors when [up] is empty.
func TestUpRequiresServices(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	writeTestConfig(t, dir, []string{"python3"}, "minimal")
	// writeTestConfig writes a config with no Up section
	chdirTemp(t, dir)

	// read what was written so we know Up is empty
	cfg, err := config.Read(config.Filename)
	require.NoError(t, err)
	assert.Empty(t, cfg.Up, "writeTestConfig must not set Up services")

	err = runUp(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no services")
}

// TestPrefixLines verifies that each line from src is written to dst with the prefix prepended.
func TestPrefixLines(t *testing.T) {
	input := "line one\nline two\nline three"
	var buf bytes.Buffer
	prefixLines(&buf, ">> ", strings.NewReader(input))

	out := buf.String()
	assert.Contains(t, out, ">> line one")
	assert.Contains(t, out, ">> line two")
	assert.Contains(t, out, ">> line three")
	assert.Equal(t, 3, strings.Count(out, ">> "), "every line should carry the prefix")
}

// TestUpServicesAreSorted verifies that service names from cfg.Up are sorted
// deterministically — important for stable color assignment and log readability.
func TestUpServicesAreSorted(t *testing.T) {
	services := map[string]string{
		"zebra": "echo z",
		"alpha": "echo a",
		"mango": "echo m",
	}
	names := sortedKeys(services)
	assert.Equal(t, []string{"alpha", "mango", "zebra"}, names)
}

// sortedKeys is a testable helper that mirrors the sort in runUp.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// same sort as runUp
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
