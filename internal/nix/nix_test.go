package nix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
)

func TestParseResolveOutputNormal(t *testing.T) {
	stdout := "/nix/store/abc-bash-5.2/bin/bash\n/nix/store/def-coreutils/bin/env\n/nix/store/abc-bash-5.2/bin:/nix/store/def-coreutils/bin"
	env, err := parseResolveOutput(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(env.BashPath, "/bash") {
		t.Errorf("BashPath wrong: %s", env.BashPath)
	}
	if !strings.HasSuffix(env.EnvPath, "/env") {
		t.Errorf("EnvPath wrong: %s", env.EnvPath)
	}
	if env.PATH == "" {
		t.Error("PATH must not be empty")
	}
}

func TestParseResolveOutputExtraLines(t *testing.T) {
	// extra lines before which output (e.g. shellHook writing to stdout)
	stdout := "note: blah blah\n/nix/store/abc-bash-5.2/bin/bash\n/nix/store/def-coreutils/bin/env\n/nix/store/abc-bash-5.2/bin:/nix/store/def-coreutils/bin"
	env, err := parseResolveOutput(stdout)
	if err != nil {
		t.Fatalf("extra lines must not break parsing: %v", err)
	}
	if !strings.HasSuffix(env.BashPath, "/bash") {
		t.Errorf("BashPath wrong with extra lines: %s", env.BashPath)
	}
}

func TestParseResolveOutputTooFewLines(t *testing.T) {
	_, err := parseResolveOutput("only one line")
	if err == nil {
		t.Error("expected error for fewer than 3 lines")
	}
}

func TestParseResolveOutputNoBashPath(t *testing.T) {
	// env present but no path ending in /bash
	stdout := "/nix/store/def-coreutils/bin/notbash\n/nix/store/def-coreutils/bin/env\n/nix/store/def-coreutils/bin"
	_, err := parseResolveOutput(stdout)
	if err == nil {
		t.Error("expected error when bash path not found")
	}
}

func TestParseResolveOutputNoNixStorePaths(t *testing.T) {
	stdout := "/usr/bin/bash\n/usr/bin/env\n/usr/bin:/usr/local/bin"
	_, err := parseResolveOutput(stdout)
	if err == nil {
		t.Error("expected error when PATH has no nix store paths")
	}
}

// TestReproducibleShellNix: the core reproducibility claim — same config, same hash, any machine.
func TestReproducibleShellNix(t *testing.T) {
	cfg := &config.Config{
		Packages:      []string{"python311", "cowsay"},
		NixpkgsCommit: "abc123",
		NixpkgsSHA256: "sha256abc",
		Profile:       "minimal",
	}
	sum1, err := GenerateShellNix(cfg, filepath.Join(t.TempDir(), "shell.nix"))
	if err != nil {
		t.Fatal(err)
	}
	sum2, err := GenerateShellNix(cfg, filepath.Join(t.TempDir(), "shell.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if sum1 != sum2 {
		t.Error("same config must produce identical shell.nix hash regardless of directory")
	}
}

func TestParseNixErrorMissingAttr(t *testing.T) {
	err := parseNixError([]byte(`error: attribute 'foobar' missing`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Errorf("expected package name in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "package not found") {
		t.Errorf("expected 'package not found', got: %v", err)
	}
}

func TestParseNixErrorGeneric(t *testing.T) {
	err := parseNixError([]byte("something went wrong"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nix-shell failed") {
		t.Errorf("expected 'nix-shell failed', got: %v", err)
	}
}

func TestFilterOutNixEnv(t *testing.T) {
	in := []string{"HOME=/home/user", "NIX_PATH=/nix/var", "PATH=/usr/bin", "NIX_CONF_DIR=/etc/nix"}
	out := filterOutNixEnv(in)
	for _, kv := range out {
		if strings.HasPrefix(kv, "NIX_") {
			t.Errorf("NIX_ var must be filtered out: %s", kv)
		}
	}
	if len(out) != 2 {
		t.Errorf("expected 2 vars after filtering, got %d", len(out))
	}
}

// TestGenerateShellNixContainsAllPackages verifies that every package in the
// config appears verbatim in the generated shell.nix buildInputs list.
func TestGenerateShellNixContainsAllPackages(t *testing.T) {
	cfg := &config.Config{
		Packages:      []string{"python311", "rustc", "cargo", "nodejs"},
		NixpkgsCommit: "abc123",
		NixpkgsSHA256: "sha256test",
		Profile:       "minimal",
	}
	outPath := filepath.Join(t.TempDir(), "shell.nix")
	if _, err := GenerateShellNix(cfg, outPath); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range cfg.Packages {
		if !strings.Contains(string(content), pkg) {
			t.Errorf("package %q missing from generated shell.nix", pkg)
		}
	}
}

// TestGenerateShellNixSkipsWriteIfUnchanged verifies that calling
// GenerateShellNix twice with the same config does not rewrite the file —
// same content means same sum means early return before WriteFile.
func TestGenerateShellNixSkipsWriteIfUnchanged(t *testing.T) {
	cfg := &config.Config{
		Packages:      []string{"git", "curl"},
		NixpkgsCommit: "deadbeef01234567890123456789012345678901",
		NixpkgsSHA256: "sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Profile:       "minimal",
	}
	outPath := filepath.Join(t.TempDir(), "shell.nix")

	_, err := GenerateShellNix(cfg, outPath)
	if err != nil {
		t.Fatal(err)
	}
	info1, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = GenerateShellNix(cfg, outPath)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}

	if !info2.ModTime().Equal(info1.ModTime()) {
		t.Error("file must not be rewritten when content is unchanged — mtime changed on second call")
	}
}

// TestGenerateShellNixPackageOrderProducesDistinctHashes documents that
// package order currently affects the generated hash. Users who reorder
// packages in lagoon.toml will trigger a cold rebuild unnecessarily.
//
// If this test starts failing (hashes become equal), it means sorting has
// been implemented — update the test to assert equality instead.
func TestGenerateShellNixPackageOrderProducesDistinctHashes(t *testing.T) {
	cfgAB := &config.Config{
		Packages: []string{"python311", "git"}, NixpkgsCommit: "abc", NixpkgsSHA256: "sha", Profile: "minimal",
	}
	cfgBA := &config.Config{
		Packages: []string{"git", "python311"}, NixpkgsCommit: "abc", NixpkgsSHA256: "sha", Profile: "minimal",
	}

	sumAB, _ := GenerateShellNix(cfgAB, filepath.Join(t.TempDir(), "shell.nix"))
	sumBA, _ := GenerateShellNix(cfgBA, filepath.Join(t.TempDir(), "shell.nix"))

	if sumAB == sumBA {
		t.Log("NOTE: package order no longer affects hash — sorting implemented, update this test")
	} else {
		t.Logf("INFO: package order is significant — ['python311','git'] != ['git','python311']. "+
			"users reordering lagoon.toml packages will trigger unnecessary cold rebuilds. "+
			"sumAB=%s sumBA=%s", sumAB, sumBA)
	}
	// this test intentionally never calls t.Error — it documents behaviour either way
}
