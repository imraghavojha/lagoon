package nix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imraghavojha/lagoon/internal/config"
)

func testCfg(packages ...string) *config.Config {
	return &config.Config{
		Packages:      packages,
		NixpkgsCommit: "aaaa1234",
		NixpkgsSHA256: "sha256xxxx",
		Profile:       "minimal",
	}
}

func TestGenerateShellNixContent(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "shell.nix")

	_, err := GenerateShellNix(testCfg("python311", "cowsay"), out)
	if err != nil {
		t.Fatalf("GenerateShellNix: %v", err)
	}

	b, _ := os.ReadFile(out)
	content := string(b)
	for _, want := range []string{"python311", "cowsay", "bash", "coreutils", "aaaa1234", "sha256xxxx"} {
		if !strings.Contains(content, want) {
			t.Errorf("shell.nix missing %q", want)
		}
	}
}

func TestGenerateShellNixReturnsSum(t *testing.T) {
	sum, err := GenerateShellNix(testCfg("cowsay"), filepath.Join(t.TempDir(), "shell.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Error("sum must not be empty")
	}
}

func TestGenerateShellNixSumChanges(t *testing.T) {
	dir := t.TempDir()
	s1, _ := GenerateShellNix(testCfg("cowsay"), filepath.Join(dir, "a.nix"))
	s2, _ := GenerateShellNix(testCfg("python311"), filepath.Join(dir, "b.nix"))
	if s1 == s2 {
		t.Error("different packages must produce different sums")
	}
}

func TestGenerateShellNixSkipsWriteIfUnchanged(t *testing.T) {
	out := filepath.Join(t.TempDir(), "shell.nix")

	GenerateShellNix(testCfg("cowsay"), out)
	info1, _ := os.Stat(out)

	time.Sleep(5 * time.Millisecond)

	GenerateShellNix(testCfg("cowsay"), out)
	info2, _ := os.Stat(out)

	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("file was rewritten despite identical content")
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

func TestParseNixErrorEmpty(t *testing.T) {
	if parseNixError([]byte("")) == nil {
		t.Fatal("expected error even for empty input")
	}
}
