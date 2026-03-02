package cmd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildModelReceivesNixLine verifies that a nixLineMsg updates lastLine and
// schedules the next waitNixLine command so the loop continues.
func TestBuildModelReceivesNixLine(t *testing.T) {
	ch := make(chan string, 1)
	m := newBuildModel(ch)

	m2, cmd := m.Update(nixLineMsg("fetching python..."))
	bm := m2.(buildModel)

	assert.Equal(t, "fetching python...", bm.lastLine, "lastLine must be updated to the received line")
	assert.NotNil(t, cmd, "should return waitNixLine cmd to keep reading from channel")
}

// TestBuildModelMultipleLinesKeepsLatest verifies that each new line overwrites
// the previous — only the most recent nix output is displayed.
func TestBuildModelMultipleLinesKeepsLatest(t *testing.T) {
	ch := make(chan string, 1)
	m := newBuildModel(ch)

	m2a, _ := m.Update(nixLineMsg("fetching bash..."))
	m = m2a.(buildModel)
	m2b, _ := m.Update(nixLineMsg("building coreutils..."))
	m = m2b.(buildModel)
	m2, _ := m.Update(nixLineMsg("copying to store..."))
	bm := m2.(buildModel)

	assert.Equal(t, "copying to store...", bm.lastLine,
		"only the latest nix line should be shown")
}

// TestBuildModelDoneQuitsProgram verifies that nixDoneMsg triggers tea.Quit
// so the spinner program exits when nix-shell finishes.
func TestBuildModelDoneQuitsProgram(t *testing.T) {
	ch := make(chan string)
	m := newBuildModel(ch)

	_, cmd := m.Update(nixDoneMsg{})
	require.NotNil(t, cmd, "done message must return a quit command")

	msg := cmd()
	assert.Equal(t, tea.QuitMsg{}, msg, "nixDoneMsg must produce tea.QuitMsg")
}

// TestBuildModelCtrlCQuitsProgram verifies that Ctrl+C exits the spinner.
func TestBuildModelCtrlCQuitsProgram(t *testing.T) {
	ch := make(chan string)
	m := newBuildModel(ch)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd)

	msg := cmd()
	assert.Equal(t, tea.QuitMsg{}, msg, "ctrl+c must produce tea.QuitMsg")
}

// TestBuildModelSpinnerTickUpdatesSpinner verifies that spinner.TickMsg is
// forwarded to the spinner sub-model and a new tick command is scheduled.
func TestBuildModelSpinnerTickUpdatesSpinner(t *testing.T) {
	ch := make(chan string)
	m := newBuildModel(ch)

	tick := spinner.TickMsg{}
	_, cmd := m.Update(tick)
	// the spinner should return a non-nil cmd to schedule the next tick
	assert.NotNil(t, cmd, "spinner TickMsg should return a new tick cmd")
}

// TestBuildModelViewContainsLastLine verifies that View() renders the most
// recent nix output line.
func TestBuildModelViewContainsLastLine(t *testing.T) {
	ch := make(chan string)
	m := newBuildModel(ch)
	m.lastLine = "downloading bash-5.2..."

	view := m.View()
	assert.Contains(t, view, "downloading bash-5.2...", "View must include the last nix output line")
}

// TestBuildModelViewTruncatesLongLines verifies that lines longer than 60 chars
// are truncated with an ellipsis so the spinner display stays compact.
func TestBuildModelViewTruncatesLongLines(t *testing.T) {
	ch := make(chan string)
	m := newBuildModel(ch)
	m.lastLine = strings.Repeat("x", 80) // 80 chars > 60 char limit

	view := m.View()
	assert.Contains(t, view, "…", "long line must be truncated with ellipsis")
	assert.NotContains(t, view, strings.Repeat("x", 80), "full 80-char string must not appear")
}

// TestBuildModelViewContainsBuildingText verifies the static UI text is present.
func TestBuildModelViewContainsBuildingText(t *testing.T) {
	ch := make(chan string)
	m := newBuildModel(ch)

	view := m.View()
	assert.Contains(t, view, "building environment", "View must contain static 'building environment' text")
}

// TestWaitNixLineReceivesFromChannel verifies that waitNixLine reads from the
// channel and wraps the value in nixLineMsg.
func TestWaitNixLineReceivesFromChannel(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "building python-3.11..."

	cmd := waitNixLine(ch)
	msg := cmd()

	assert.Equal(t, nixLineMsg("building python-3.11..."), msg)
}

// TestWaitNixLineChannelClosedReturnsDoneMsg verifies that a closed channel
// produces nixDoneMsg (not a panic or empty message).
func TestWaitNixLineChannelClosedReturnsDoneMsg(t *testing.T) {
	ch := make(chan string)
	close(ch)

	cmd := waitNixLine(ch)
	msg := cmd()

	assert.Equal(t, nixDoneMsg{}, msg, "closed channel must return nixDoneMsg")
}
