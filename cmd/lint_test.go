package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
