package preflight

import (
	"strings"
	"testing"
)

func TestCheckBwrapMissing(t *testing.T) {
	t.Setenv("PATH", "")
	err := checkBwrap()
	if err == nil {
		t.Fatal("expected error when bwrap not on PATH")
	}
	if !strings.Contains(err.Error(), "bubblewrap not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckNixMissing(t *testing.T) {
	t.Setenv("PATH", "")
	err := checkNix()
	if err == nil {
		t.Fatal("expected error when nix-shell not on PATH")
	}
	if !strings.Contains(err.Error(), "nix not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestCheckUsernsMissingFile: on macOS /proc doesn't exist so the file is
// absent — checkUserns must return nil (absent means enabled by default).
func TestCheckUsernsMissingFile(t *testing.T) {
	if err := checkUserns(); err != nil {
		// only acceptable failure is on linux where the file exists and is "0"
		if !strings.Contains(err.Error(), "user namespaces are disabled") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}
