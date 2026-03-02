package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers ----------------------------------------------------------------

// sendKey simulates a key press through the model's Update().
func sendKey(m searchModel, key string) (searchModel, tea.Cmd) {
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return m2.(searchModel), cmd
}

// sendMsg pushes an arbitrary message through Update().
func sendMsg(m searchModel, msg tea.Msg) (searchModel, tea.Cmd) {
	m2, cmd := m.Update(msg)
	return m2.(searchModel), cmd
}

// setInput forces the input field value without going through key events.
// bubbletea textinput exposes SetValue for this purpose.
func setInput(m searchModel, v string) searchModel {
	m.input.SetValue(v)
	return m
}

// model state tests ------------------------------------------------------

func TestSearchModelInit(t *testing.T) {
	m := newSearchModel()
	assert.Empty(t, m.results, "fresh model should have no results")
	assert.Empty(t, m.selected, "fresh model should have no selected packages")
	assert.Equal(t, 0, m.cursor, "cursor should start at 0")
	assert.Empty(t, m.searchErr, "no error on init")
}

// TestSearchModelTypingTriggersDebounceCmd verifies that typing a character
// causes Update() to return a non-nil Cmd (the debounce timer).
func TestSearchModelTypingTriggersDebounceCmd(t *testing.T) {
	m := newSearchModel()
	_, cmd := sendKey(m, "p")
	assert.NotNil(t, cmd, "typing should return a debounce cmd")
}

// TestSearchModelMatchingDebounceTriggerseFetch verifies that when a
// pkgDebounce message matches the current input text, a fetch is triggered.
func TestSearchModelMatchingDebounceTriggerseFetch(t *testing.T) {
	m := setInput(newSearchModel(), "python")
	// debounce with same text as current input — should trigger fetch
	m2, cmd := sendMsg(m, pkgDebounce("python"))
	_ = m2
	assert.NotNil(t, cmd, "matching debounce should trigger fetchPkgsCmd")
}

// TestSearchModelStaleDebounceIgnored verifies that a debounce message whose
// query no longer matches the current input is dropped (no fetch triggered).
// This is the core of the "disappearing recommendations" bug: if the user
// types "py" then immediately types "t" (input is now "pyt"), the "py"
// debounce message should NOT trigger a fetch.
func TestSearchModelStaleDebounceIgnored(t *testing.T) {
	m := setInput(newSearchModel(), "pyt")
	// debounce from an earlier keystroke — input has moved on
	_, cmd := sendMsg(m, pkgDebounce("py"))
	assert.Nil(t, cmd, "stale debounce should be dropped — not trigger a fetch")
}

// TestSearchModelEmptyDebounceIgnored verifies that a debounce for an empty
// string (user deleted all text) does not trigger a fetch.
func TestSearchModelEmptyDebounceIgnored(t *testing.T) {
	m := newSearchModel() // input is empty
	_, cmd := sendMsg(m, pkgDebounce(""))
	assert.Nil(t, cmd, "empty debounce should not trigger fetch")
}

// TestSearchModelResultsPopulate verifies that pkgResultsMsg sets the
// results list, resets the cursor to 0, and clears any previous error.
func TestSearchModelResultsPopulate(t *testing.T) {
	m := newSearchModel()
	m.searchErr = "previous error"
	m.cursor = 2

	pkgs := []nixPkg{{"python3", "Python 3"}, {"python311", "Python 3.11"}}
	m2, _ := sendMsg(m, pkgResultsMsg(pkgs))

	assert.Len(t, m2.results, 2, "results should be populated")
	assert.Equal(t, 0, m2.cursor, "cursor should reset to 0 on new results")
	assert.Empty(t, m2.searchErr, "error should be cleared on results")
	assert.Equal(t, "python3", m2.results[0].name)
}

// TestSearchModelCursorNavigation verifies up/down navigation with boundary
// checks — cursor must not go below 0 or above len(results)-1.
func TestSearchModelCursorNavigation(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{"a", ""}, {"b", ""}, {"c", ""}}
	m.cursor = 0

	// move down twice
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, m.cursor)
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.cursor)

	// cannot go past last result
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.cursor, "cursor should not exceed last index")

	// move back up
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, m.cursor)

	// cannot go below 0
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyUp})
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, m.cursor, "cursor should not go below 0")
}

// TestSearchModelEnterSelectsHighlightedResult verifies that pressing Enter
// when the cursor is on a result adds that result's name to selected[].
func TestSearchModelEnterSelectsHighlightedResult(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{"python3", "Python 3"}, {"nodejs", "Node.js"}}
	m.cursor = 1 // pointing at nodejs

	m2, _ := sendMsg(m, tea.KeyMsg{Type: tea.KeyEnter})

	require.Len(t, m2.selected, 1)
	assert.Equal(t, "nodejs", m2.selected[0], "enter should select highlighted result")
	assert.Empty(t, m2.results, "results should be cleared after selection")
	assert.Equal(t, 0, m2.cursor, "cursor should reset after selection")
	assert.Empty(t, m2.input.Value(), "input should be cleared after selection")
}

// TestSearchModelEnterWithNoResultsAddsTypedText verifies that pressing Enter
// when results list is empty adds the raw typed text as a package name.
func TestSearchModelEnterWithNoResultsAddsTypedText(t *testing.T) {
	m := setInput(newSearchModel(), "mypkg")
	m.results = nil

	m2, _ := sendMsg(m, tea.KeyMsg{Type: tea.KeyEnter})

	require.Len(t, m2.selected, 1)
	assert.Equal(t, "mypkg", m2.selected[0], "typed text should be added when no results")
}

// TestSearchModelEnterWithBlankInputDoesNothing verifies that pressing Enter
// with empty input and no results does not add an empty string to selected[].
func TestSearchModelEnterWithBlankInputDoesNothing(t *testing.T) {
	m := newSearchModel()
	m.results = nil

	m2, _ := sendMsg(m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Empty(t, m2.selected, "enter on blank input should not add empty package")
}

// TestSearchModelEscapeQuitsWithSelectionIntact verifies that Esc quits the
// program while preserving whatever has been selected so far.
func TestSearchModelEscapeQuitsWithSelectionIntact(t *testing.T) {
	m := newSearchModel()
	m.selected = []string{"python3", "git"}

	_, cmd := sendMsg(m, tea.KeyMsg{Type: tea.KeyEsc})
	// the returned Cmd should be tea.Quit — execute it and verify
	require.NotNil(t, cmd, "esc should return a quit cmd")
	msg := cmd()
	assert.Equal(t, tea.Quit(), msg, "esc should return tea.Quit message")
}

// TestSearchModelErrorMessageDisplayed verifies that pkgErrMsg sets the
// searchErr field, clears results, and that View() renders the error string.
func TestSearchModelErrorMessageDisplayed(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{"python3", ""}}

	m2, _ := sendMsg(m, pkgErrMsg("connection refused"))

	assert.Equal(t, "connection refused", m2.searchErr)
	assert.Empty(t, m2.results, "error message should clear results")
	assert.Contains(t, m2.View(), "connection refused", "View should show error text")
}

// TestSearchModelTypingClearsError verifies that typing after an error clears
// the error display — the user is searching again.
func TestSearchModelTypingClearsError(t *testing.T) {
	m := newSearchModel()
	m.searchErr = "previous error"

	m2, _ := sendKey(m, "p")

	assert.Empty(t, m2.searchErr, "typing should clear previous search error")
}

// TestSearchModelViewShowsCursorOnHighlightedRow verifies that View() renders
// the cursor arrow (▶) only on the row the cursor is pointing at.
func TestSearchModelViewShowsCursorOnHighlightedRow(t *testing.T) {
	m := newSearchModel()
	m.results = []nixPkg{{"alpha", ""}, {"beta", ""}, {"gamma", ""}}
	m.cursor = 1

	view := m.View()

	lines := strings.Split(view, "\n")
	var resultLines []string
	for _, l := range lines {
		if strings.Contains(l, "alpha") || strings.Contains(l, "beta") || strings.Contains(l, "gamma") {
			resultLines = append(resultLines, l)
		}
	}

	require.Len(t, resultLines, 3, "all three results should appear in View()")
	assert.NotContains(t, resultLines[0], "▶", "alpha (index 0) should not have cursor")
	assert.Contains(t, resultLines[1], "▶", "beta (index 1, cursor) should have ▶")
	assert.NotContains(t, resultLines[2], "▶", "gamma (index 2) should not have cursor")
}

// TestSearchModelViewShowsSelectedPackages verifies that selected packages
// appear in View() after being added.
func TestSearchModelViewShowsSelectedPackages(t *testing.T) {
	m := newSearchModel()
	m.selected = []string{"python3", "git", "ripgrep"}

	view := m.View()

	assert.Contains(t, view, "python3")
	assert.Contains(t, view, "git")
	assert.Contains(t, view, "ripgrep")
}

// TestSearchModelViewEmptyResultsNoArrow verifies that View() renders no
// cursor arrow when the results list is empty.
func TestSearchModelViewEmptyResultsNoArrow(t *testing.T) {
	m := newSearchModel()
	m.results = nil

	view := m.View()

	assert.NotContains(t, view, "▶", "no arrow when results are empty")
}

// api tests (using mock server) ------------------------------------------

// TestQueryNixpkgsReturnsResults verifies that queryNixpkgs() correctly
// parses the Elasticsearch response and returns the package list.
func TestQueryNixpkgsReturnsResults(t *testing.T) {
	srv := httptest.NewServer(searchResponse(
		nixPkg{"python3", "Python 3 interpreter"},
		nixPkg{"python311", "Python 3.11"},
	))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	pkgs, err := queryNixpkgs("python")
	require.NoError(t, err)
	require.Len(t, pkgs, 2)
	assert.Equal(t, "python3", pkgs[0].name)
	assert.Equal(t, "Python 3 interpreter", pkgs[0].desc)
}

// TestQueryNixpkgsMalformedJSON verifies that a garbage JSON response from the
// API returns an error rather than silently returning empty results or panicking.
func TestQueryNixpkgsMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is not json {{{"))
	}))
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	_, err := queryNixpkgs("python")
	assert.Error(t, err, "malformed JSON should return an error")
}

// TestQueryNixpkgsEmptyResults verifies that an empty hits array is handled
// gracefully — returns empty slice, not nil, not error.
func TestQueryNixpkgsEmptyResults(t *testing.T) {
	srv := httptest.NewServer(searchResponse()) // no packages
	defer srv.Close()

	old := nixSearchURL
	nixSearchURL = srv.URL
	defer func() { nixSearchURL = old }()

	pkgs, err := queryNixpkgs("notarealpackage99999")
	require.NoError(t, err)
	assert.Empty(t, pkgs, "empty hits should return empty slice")
}

// TestSearchModelStaleResultsArriveAfterInputChanged documents a known bug:
// results from a fetch triggered for query "python" arrive AFTER the user
// has updated the input to "python3". The current code does not discard these
// stale results — they get displayed even though they belong to an older query.
//
// This test is expected to FAIL with the current implementation. It documents
// the desired behaviour (stale results should not be shown).
func TestSearchModelStaleResultsArriveAfterInputChanged(t *testing.T) {
	// user typed "python", fetch was triggered, then typed more → "python3"
	m := setInput(newSearchModel(), "python3")

	// stale results arrive from the earlier "python" fetch
	staleResults := []nixPkg{{"python", "Python"}, {"python2", "Python 2"}}
	m2, _ := sendMsg(m, pkgResultsMsg(staleResults))

	// desired: stale results should be discarded when input has changed
	// actual: the code sets m.results unconditionally regardless of query
	if len(m2.results) > 0 {
		t.Errorf("BUG: stale results from 'python' query are displayed when input is 'python3'. "+
			"got %d results, want 0. the searchModel has no way to correlate results to their query.", len(m2.results))
	}
}
