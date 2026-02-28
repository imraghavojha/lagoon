package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var nixSearchURL = "https://search.nixos.org/backend/latest-42-nixpkgs-unstable/nix-packages/_search"

type nixPkg struct{ name, desc string }

type (
	pkgResultsMsg []nixPkg
	pkgDebounce   string
)

type searchModel struct {
	input    textinput.Model
	results  []nixPkg
	cursor   int
	selected []string
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
		m.results = []nixPkg(msg)
		m.cursor = 0
		return m, nil
	}

	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
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
		pkgs, _ := queryNixpkgs(q)
		return pkgResultsMsg(pkgs)
	}
}

func queryNixpkgs(q string) ([]nixPkg, error) {
	body := fmt.Sprintf(`{"query":{"multi_match":{"query":%q,"fields":["package_attr_name^9","package_pname^6","package_description^1"]}},"size":8}`, q)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", nixSearchURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("aWVSALXpZv", "X8gPHnzL52wFEekuxsfQ9cSh")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
		return nil, err
	}
	pkgs := make([]nixPkg, 0, len(result.Hits.Hits))
	for _, h := range result.Hits.Hits {
		pkgs = append(pkgs, nixPkg{name: h.Source.Name, desc: h.Source.Desc})
	}
	return pkgs, nil
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
	return final.(searchModel).selected, nil
}
