package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type buildModel struct {
	spinner  spinner.Model
	progress <-chan string
	lastLine string
	start    time.Time
}

type (
	nixLineMsg string
	nixDoneMsg struct{}
)

func newBuildModel(progress <-chan string) buildModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	return buildModel{spinner: s, progress: progress, start: time.Now()}
}

func (m buildModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitNixLine(m.progress))
}

func waitNixLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nixDoneMsg{}
		}
		return nixLineMsg(line)
	}
}

func (m buildModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case nixDoneMsg:
		return m, tea.Quit
	case nixLineMsg:
		m.lastLine = string(msg)
		return m, waitNixLine(m.progress)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m buildModel) View() string {
	elapsed := time.Since(m.start).Round(time.Second)
	line := strings.TrimSpace(m.lastLine)
	if len(line) > 60 {
		line = line[:60] + "…"
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return fmt.Sprintf("\n  %s building environment  %s  %s\n\n",
		m.spinner.View(),
		dim.Render(elapsed.String()),
		dim.Render(line),
	)
}
