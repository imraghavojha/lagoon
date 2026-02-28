package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQueryNixpkgsParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hits":{"hits":[
			{"_source":{"package_attr_name":"python311","package_description":"Python 3.11"}},
			{"_source":{"package_attr_name":"python312","package_description":"Python 3.12"}}
		]}}`))
	}))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	pkgs, err := queryNixpkgs("python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(pkgs))
	}
	if pkgs[0].name != "python311" {
		t.Errorf("expected python311, got %s", pkgs[0].name)
	}
	if pkgs[1].desc != "Python 3.12" {
		t.Errorf("expected 'Python 3.12', got %s", pkgs[1].desc)
	}
}

func TestQueryNixpkgsEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hits":{"hits":[]}}`))
	}))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	pkgs, err := queryNixpkgs("zzznomatch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Errorf("expected 0 results, got %d", len(pkgs))
	}
}

func TestQueryNixpkgsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	// soft failure — returns nil/err, must not panic
	pkgs, _ := queryNixpkgs("python")
	if pkgs != nil {
		t.Errorf("expected nil on bad JSON, got %v", pkgs)
	}
}

func TestSearchModelEnterAddsFromResults(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{name: "python311", desc: "Python 3.11"}, {name: "python312", desc: "Python 3.12"}}
	m.cursor = 0

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := m2.(searchModel)
	if len(final.selected) != 1 || final.selected[0] != "python311" {
		t.Errorf("expected [python311], got %v", final.selected)
	}
	if len(final.results) != 0 {
		t.Error("results must be cleared after selection")
	}
}

func TestSearchModelEnterAddsTypedWhenNoResults(t *testing.T) {
	m := newSearchModel()
	m.input.SetValue("cowsay")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := m2.(searchModel)
	if len(final.selected) != 1 || final.selected[0] != "cowsay" {
		t.Errorf("expected [cowsay], got %v", final.selected)
	}
}

func TestSearchModelCursorNavigation(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{name: "a"}, {name: "b"}, {name: "c"}}
	m.cursor = 0

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(searchModel).cursor != 1 {
		t.Error("down should move cursor to 1")
	}
	m3, _ := m2.(searchModel).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m3.(searchModel).cursor != 0 {
		t.Error("up should move cursor back to 0")
	}
}

func TestSearchModelCursorBounds(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{name: "only"}}
	m.cursor = 0

	// can't go below 0
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m2.(searchModel).cursor != 0 {
		t.Error("cursor must not go below 0")
	}
	// can't go past last result
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m3.(searchModel).cursor != 0 {
		t.Error("cursor must not exceed last result index")
	}
}
