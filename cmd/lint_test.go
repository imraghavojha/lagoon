package cmd

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
)

// searchResponse returns a handler that returns the given packages as hits.
func searchResponse(pkgs ...nixPkg) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hits := ""
		for i, p := range pkgs {
			if i > 0 {
				hits += ","
			}
			hits += `{"_source":{"package_attr_name":"` + p.name + `","package_description":"` + p.desc + `"}}`
		}
		w.Write([]byte(`{"hits":{"hits":[` + hits + `]}}`))
	}
}

func TestLintPackageFound(t *testing.T) {
	srv := httptest.NewServer(searchResponse(nixPkg{"python311", "Python 3.11"}))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	results, err := queryNixpkgs("python311")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range results {
		if r.name == "python311" {
			found = true
		}
	}
	if !found {
		t.Error("expected python311 to be found")
	}
}

func TestLintPackageNotFound(t *testing.T) {
	srv := httptest.NewServer(searchResponse(nixPkg{"python312", "Python 3.12"}))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	results, err := queryNixpkgs("python311")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range results {
		if r.name == "python311" {
			found = true
		}
	}
	if found {
		t.Error("python311 should not be found when server only returns python312")
	}
}

func writeTempConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := config.Write(filepath.Join(dir, config.Filename), cfg); err != nil {
		t.Fatal(err)
	}
}

func TestLintStructuralEmptyPackages(t *testing.T) {
	writeTempConfig(t, &config.Config{
		NixpkgsCommit: "abc",
		NixpkgsSHA256: "sha",
		Profile:       "minimal",
	})
	if err := runLint(nil, nil); err == nil {
		t.Error("expected error for empty packages list")
	}
}

func TestLintStructuralMissingCommit(t *testing.T) {
	writeTempConfig(t, &config.Config{
		Packages:      []string{"cowsay"},
		NixpkgsSHA256: "sha",
		Profile:       "minimal",
	})
	if err := runLint(nil, nil); err == nil {
		t.Error("expected error for missing nixpkgs_commit")
	}
}

func TestLintStructuralInvalidProfile(t *testing.T) {
	writeTempConfig(t, &config.Config{
		Packages:      []string{"cowsay"},
		NixpkgsCommit: "abc",
		NixpkgsSHA256: "sha",
		Profile:       "badprofile",
	})
	if err := runLint(nil, nil); err == nil {
		t.Error("expected error for invalid profile")
	}
}

func TestLintNoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := runLint(nil, nil); err == nil {
		t.Error("expected error when lagoon.toml is missing")
	}
}

func TestLintOfflineDegrades(t *testing.T) {
	// simulate offline by using an unreachable address
	old := nixSearchURL
	nixSearchURL = "http://127.0.0.1:1" // nothing listening there
	defer func() { nixSearchURL = old }()

	_, err := queryNixpkgs("python311")
	if err == nil {
		t.Error("expected error on connection refused")
	}
}
