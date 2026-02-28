package cmd

import (
	"testing"
)

func TestImportFileNotFound(t *testing.T) {
	err := runImport(nil, []string{"/nonexistent/lagoon-archive.nar"})
	if err == nil {
		t.Error("expected error for missing import file")
	}
}
