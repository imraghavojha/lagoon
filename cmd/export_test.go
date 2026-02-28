package cmd

import (
	"testing"
)

func TestExportNoConfig(t *testing.T) {
	// no lagoon.toml in empty temp dir — must return an error
	t.Chdir(t.TempDir())
	err := runExport(nil, nil)
	if err == nil {
		t.Error("expected error when lagoon.toml is missing")
	}
}
