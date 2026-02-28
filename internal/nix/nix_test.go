package nix

import (
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
