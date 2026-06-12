package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// searchSchemaVersion is the elasticsearch mapping version search.nixos.org
// currently serves. NixOS rotates it every few releases, so queryNixpkgs
// probes forward from here when the index has moved.
const searchSchemaVersion = 48

var (
	searchURLOverride = os.Getenv("LAGOON_NIX_SEARCH_URL")

	searchURLMu       sync.Mutex
	resolvedSearchURL string // first URL that answered; reused for the process
)

func candidateSearchURLs() []string {
	if searchURLOverride != "" {
		return []string{searchURLOverride}
	}
	searchURLMu.Lock()
	resolved := resolvedSearchURL
	searchURLMu.Unlock()
	if resolved != "" {
		return []string{resolved}
	}
	urls := make([]string, 0, 13)
	for v := searchSchemaVersion; v <= searchSchemaVersion+12; v++ {
		urls = append(urls, fmt.Sprintf("https://search.nixos.org/backend/latest-%d-nixos-unstable/_search", v))
	}
	return urls
}

func rememberSearchURL(url string) {
	searchURLMu.Lock()
	resolvedSearchURL = url
	searchURLMu.Unlock()
}

type nixPkg struct{ name, desc string }

type (
	pkgResultsMsg struct {
		query string
		pkgs  []nixPkg
	}
	pkgDebounce string
	pkgErrMsg   string
)

type searchModel struct {
	input    textinput.Model
	results  []nixPkg
	cursor   int
	selected []string
	searchErr string
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "type to search nixpkgs..."
	ti.CharLimit = 64
	ti.Focus()
	return searchModel{input: ti}
}

func (m searchModel) Init() tea.Cmd { return textinput.Blink }

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.cursor < len(m.results) {
				m.selected = append(m.selected, m.results[m.cursor].name)
			} else if v := strings.TrimSpace(m.input.Value()); v != "" {
				m.selected = append(m.selected, v)
			}
			m.results = nil
			m.cursor = 0
			m.input.SetValue("")
			return m, nil
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
		}
	case pkgDebounce:
		if string(msg) == m.input.Value() && m.input.Value() != "" {
			return m, fetchPkgsCmd(m.input.Value())
		}
	case pkgResultsMsg:
		// discard results that arrived for an older query
		if msg.query != m.input.Value() {
			return m, nil
		}
		m.results = msg.pkgs
		m.searchErr = ""
		m.cursor = 0
		return m, nil
	case pkgErrMsg:
		m.results = nil
		m.searchErr = string(msg)
		return m, nil
	}

	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		m.searchErr = ""
		return m, tea.Batch(cmd, pkgDebounceCmd(m.input.Value()))
	}
	return m, cmd
}

func pkgDebounceCmd(q string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(300 * time.Millisecond)
		return pkgDebounce(q)
	}
}

func fetchPkgsCmd(q string) tea.Cmd {
	return func() tea.Msg {
		pkgs, err := queryNixpkgs(q)
		if err != nil {
			return pkgErrMsg(err.Error())
		}
		return pkgResultsMsg{query: q, pkgs: pkgs}
	}
}

func queryNixpkgs(q string) ([]nixPkg, error) {
	// the index mixes packages and options — restrict to packages
	body := fmt.Sprintf(`{"query":{"bool":{"must":[{"term":{"type":"package"}},{"multi_match":{"query":%q,"fields":["package_attr_name^9","package_pname^6","package_description^1"]}}]}},"size":8}`, q)
	var lastErr error
	for _, url := range candidateSearchURLs() {
		pkgs, indexGone, err := doNixSearch(url, body)
		if err == nil {
			rememberSearchURL(url)
			return pkgs, nil
		}
		lastErr = err
		if !indexGone {
			return nil, err // network/auth problem — probing won't help
		}
	}
	return nil, lastErr
}

// doNixSearch posts one search request. indexGone reports a 404 (rotated
// index), which callers treat as "try the next schema version".
func doNixSearch(url, body string) (pkgs []nixPkg, indexGone bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	// public read-only credentials baked into search.nixos.org's frontend
	req.SetBasicAuth("aWVSALXpZv", "X8gPHnzL52wFEekuxsfQ9cSh")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, true, fmt.Errorf("nixpkgs search index moved — update lagoon or set LAGOON_NIX_SEARCH_URL")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("nixpkgs search returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Name string `json:"package_attr_name"`
					Desc string `json:"package_description"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, err
	}
	pkgs = make([]nixPkg, 0, len(result.Hits.Hits))
	for _, h := range result.Hits.Hits {
		pkgs = append(pkgs, nixPkg{name: h.Source.Name, desc: h.Source.Desc})
	}
	return pkgs, false, nil
}

var (
	pkgCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	pkgDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	pkgSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func (m searchModel) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n  search packages: %s\n\n", m.input.View())
	for i, p := range m.results {
		desc := p.desc
		if len(desc) > 52 {
			desc = desc[:52] + "…"
		}
		if i == m.cursor {
			fmt.Fprintf(&b, "  %s %-22s %s\n", pkgCursor.Render("▶"), pkgCursor.Render(p.name), pkgDimStyle.Render(desc))
		} else {
			fmt.Fprintf(&b, "    %-22s %s\n", p.name, pkgDimStyle.Render(desc))
		}
	}
	if m.searchErr != "" {
		fmt.Fprintf(&b, "\n  %s\n", fail("✗")+" search failed: "+m.searchErr)
	}
	if len(m.selected) > 0 {
		fmt.Fprintf(&b, "\n  %s %s\n", pkgSelStyle.Render("selected:"), strings.Join(m.selected, " "))
	}
	fmt.Fprintf(&b, "\n  %s\n", pkgDimStyle.Render("↑↓ navigate • enter to add • esc when done"))
	return b.String()
}

// RunPackageSearch runs the live nixpkgs search TUI and returns chosen package names.
func RunPackageSearch() ([]string, error) {
	prog := tea.NewProgram(newSearchModel())
	final, err := prog.Run()
	if err != nil {
		return nil, err
	}
	sm, ok := final.(searchModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type returned: %T", final)
	}
	return sm.selected, nil
}
