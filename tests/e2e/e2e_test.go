// e2e tests exercise the lagoon binary end-to-end.
// requires a compiled 'lagoon' binary in $PATH or $LAGOON_BIN.
// tests that need bwrap or nix-shell skip automatically when those aren't present.
// slow nix builds require LAGOON_SLOW_E2E=1.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctxWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// lagoonBin returns the path to the lagoon binary under test.
func lagoonBin(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("LAGOON_BIN"); v != "" {
		return v
	}
	bin, err := exec.LookPath("lagoon")
	if err != nil {
		t.Skip("skipping e2e: lagoon binary not found in PATH (set LAGOON_BIN=...)")
	}
	return bin
}

// skipIfMissing skips when a required binary is absent.
func skipIfMissing(t *testing.T, bins ...string) {
	t.Helper()
	for _, b := range bins {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("skipping e2e: %s not found in PATH", b)
		}
	}
}

// run executes lagoon with args in the given directory and returns combined output.
func run(t *testing.T, dir string, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	bin := lagoonBin(t)
	ctx, cancel := newCtx(timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// newCtx returns a context with the given timeout; falls back to 5m if zero.
func newCtx(d time.Duration) (context.Context, context.CancelFunc) {
	if d == 0 {
		d = 5 * time.Minute
	}
	return ctxWithTimeout(d)
}

// writeCfg writes a real lagoon.toml with the pinned nixpkgs commit.
func writeCfg(t *testing.T, dir string, packages []string, profile string) {
	t.Helper()
	cfg := &config.Config{
		Packages:      packages,
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       profile,
	}
	require.NoError(t, config.Write(filepath.Join(dir, config.Filename), cfg))
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

// TestE2EVersionCommand verifies the binary runs and outputs sensible version info.
func TestE2EVersionCommand(t *testing.T) {
	out, err := run(t, t.TempDir(), 10*time.Second, "version")
	require.NoError(t, err, "lagoon version must exit 0")
	assert.True(t,
		strings.Contains(out, "lagoon") || strings.Contains(out, "nixpkgs") || strings.Contains(out, "commit"),
		"version output must contain version/nixpkgs info, got: %s", out)
}

// TestE2ELintValidConfig verifies that lint passes on a structurally valid config
// (structural checks run without network; package lookup is best-effort).
func TestE2ELintValidConfig(t *testing.T) {
	skipIfMissing(t, "bwrap", "nix-shell")

	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3"}, "minimal")

	out, err := run(t, dir, 30*time.Second, "check")
	// lint may fail due to network issues but must not panic
	if err != nil {
		assert.NotContains(t, out, "panic:", "lint must never panic")
		assert.NotContains(t, out, "runtime error:", "lint must never panic")
		t.Logf("lint returned error (may be offline): %v\noutput: %s", err, out)
	} else {
		assert.Contains(t, out, "python3", "lint output must mention the package")
	}
}

// TestE2ELintEmptyPackages verifies lint rejects an empty package list immediately
// (no network needed — structural validation runs first).
func TestE2ELintEmptyPackages(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, []string{}, "minimal")

	out, err := run(t, dir, 15*time.Second, "check")
	assert.Error(t, err, "lint must exit non-zero for empty packages")
	assert.Contains(t, out, "packages list is empty")
}

// TestE2ELintDuplicatePackages verifies lint rejects duplicate package entries.
func TestE2ELintDuplicatePackages(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3", "git", "python3"}, "minimal")

	out, err := run(t, dir, 15*time.Second, "check")
	assert.Error(t, err, "lint must exit non-zero for duplicate packages")
	assert.Contains(t, out, "duplicate package")
}

// TestE2ELintNoConfig verifies lint exits non-zero when lagoon.toml is absent.
func TestE2ELintNoConfig(t *testing.T) {
	dir := t.TempDir() // no lagoon.toml written

	_, err := run(t, dir, 10*time.Second, "check")
	assert.Error(t, err, "lint must exit non-zero when lagoon.toml is missing")
}

// TestE2EStatusNoConfig verifies status exits 0 but prints a warning when
// lagoon.toml is absent (status is informational, not a gate).
func TestE2EStatusNoConfig(t *testing.T) {
	dir := t.TempDir()

	out, err := run(t, dir, 10*time.Second, "ps")
	assert.NoError(t, err, "status must exit 0 even without lagoon.toml")
	assert.Contains(t, out, "lagoon init", "status must suggest running lagoon init")
}

// TestE2EStatusNotCached verifies status reports "not cached" for a fresh config.
func TestE2EStatusNotCached(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3"}, "minimal")

	out, err := run(t, dir, 10*time.Second, "ps")
	assert.NoError(t, err, "status must exit 0 on cache miss")
	assert.Contains(t, out, "not cached", "status must report not cached")
}

// TestE2ECleanNoCache verifies clean exits 0 when no cache exists.
func TestE2ECleanNoCache(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3"}, "minimal")

	out, err := run(t, dir, 10*time.Second, "rm")
	assert.NoError(t, err, "clean must exit 0 when no cache exists")
	_ = out // message about no cache is fine
}

// TestE2ERunNoConfig verifies that lagoon run fails gracefully when there is no
// lagoon.toml — it must print an error, not panic.
func TestE2ERunNoConfig(t *testing.T) {
	skipIfMissing(t, "bwrap", "nix-shell")
	dir := t.TempDir()

	out, err := run(t, dir, 15*time.Second, "run", "echo", "hello")
	assert.Error(t, err, "lagoon run must fail without lagoon.toml")
	assert.NotContains(t, out, "panic:", "lagoon run must not panic without config")
}

// TestE2EShellWithRealNix performs a full cold→warm cycle with a real nix build.
// This is the most valuable e2e test but also the slowest (10-60min on ARM cold).
// It is skipped unless the LAGOON_SLOW_E2E=1 env var is set.
func TestE2EShellWithRealNix(t *testing.T) {
	if os.Getenv("LAGOON_SLOW_E2E") != "1" {
		t.Skip("skipping slow nix build test — set LAGOON_SLOW_E2E=1 to run")
	}
	skipIfMissing(t, "bwrap", "nix-shell")

	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3"}, "minimal")

	// cold start: run python3 --version inside the sandbox
	t.Log("cold start: building nix environment (may take 10-60min on ARM)...")
	start := time.Now()
	out, err := run(t, dir, 90*time.Minute, "run", "python3", "--version")
	coldDur := time.Since(start)
	require.NoError(t, err, "cold start lagoon run must succeed\noutput: %s", out)
	assert.Contains(t, out, "Python 3", "python3 --version must print Python 3.x")
	t.Logf("cold start: %v", coldDur)

	// warm start: run again — should use cache and be fast
	start = time.Now()
	out, err = run(t, dir, 30*time.Second, "run", "python3", "--version")
	warmDur := time.Since(start)
	require.NoError(t, err, "warm start lagoon run must succeed\noutput: %s", out)
	assert.Contains(t, out, "Python 3")
	assert.Less(t, warmDur, 10*time.Second, "warm start must complete in <10s")
	t.Logf("warm start: %v", warmDur)

	// status should now show cached
	out, err = run(t, dir, 10*time.Second, "ps")
	require.NoError(t, err)
	assert.Contains(t, out, "cached", "status must show cached after successful shell")

	// env var passthrough
	out, err = run(t, dir, 30*time.Second, "shell", "-e", "MY_VAR=hello_lagoon", "--cmd", "echo $MY_VAR")
	require.NoError(t, err, "env var passthrough must work")
	assert.Contains(t, out, "hello_lagoon", "-e flag must inject env var into sandbox")

	// clean removes cache
	_, err = run(t, dir, 10*time.Second, "rm")
	require.NoError(t, err)
	out, _ = run(t, dir, 10*time.Second, "ps")
	assert.Contains(t, out, "not cached", "status must show not cached after clean")
}

// TestE2EEnvVarPassthrough verifies that -e KEY=VALUE injects the variable
// without a full nix build (requires a warm cache to be fast).
func TestE2EEnvVarPassthrough(t *testing.T) {
	if os.Getenv("LAGOON_SLOW_E2E") != "1" {
		t.Skip("requires a warm nix cache — set LAGOON_SLOW_E2E=1 to run")
	}
	skipIfMissing(t, "bwrap", "nix-shell")

	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3"}, "minimal")

	out, err := run(t, dir, 30*time.Second, "shell", "-e", "LAGOON_TEST_VAR=it_works", "--cmd", "echo $LAGOON_TEST_VAR")
	require.NoError(t, err)
	assert.Contains(t, out, "it_works", "-e flag must make env var visible inside sandbox")
}

// TestE2EUpNoConfig verifies that lagoon up fails when lagoon.toml is absent.
func TestE2EUpNoConfig(t *testing.T) {
	dir := t.TempDir()

	_, err := run(t, dir, 10*time.Second, "up")
	assert.Error(t, err, "up must exit non-zero when lagoon.toml is missing")
}

// TestE2EUpNoServices verifies that lagoon up fails when [up] section is absent.
func TestE2EUpNoServices(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, []string{"python3"}, "minimal")
	// writeCfg does not write an [up] section

	_, err := run(t, dir, 10*time.Second, "up")
	assert.Error(t, err, "up must exit non-zero when no services are defined")
}

// TestE2EDockerNoConfig verifies that lagoon docker fails when lagoon.toml is absent.
func TestE2EDockerNoConfig(t *testing.T) {
	dir := t.TempDir() // no lagoon.toml

	out, err := run(t, dir, 10*time.Second, "docker")
	assert.Error(t, err, "docker must exit non-zero when lagoon.toml is missing")
	assert.Contains(t, out, "lagoon init", "must tell user to run lagoon init")
}
