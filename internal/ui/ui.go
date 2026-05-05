package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	Accent = lipgloss.Color("12")
	Good   = lipgloss.Color("10")
	Warn   = lipgloss.Color("11")
	Bad    = lipgloss.Color("9")
	Muted  = lipgloss.Color("8")

	Title = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Dim   = lipgloss.NewStyle().Foreground(Muted)
	OK    = lipgloss.NewStyle().Foreground(Good).Bold(true)
	Hot   = lipgloss.NewStyle().Foreground(Warn).Bold(true)
	Err   = lipgloss.NewStyle().Foreground(Bad).Bold(true)
)

func Card(title string, lines ...string) string {
	body := strings.Join(lines, "\n")
	if body != "" {
		body = "\n" + body
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Accent).
		Padding(0, 1).
		Render(Title.Render(title) + body)
}

func Chip(label, value string, color lipgloss.Color) string {
	labelStyle := lipgloss.NewStyle().Foreground(Muted)
	valueStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	return labelStyle.Render(label+": ") + valueStyle.Render(value)
}

func Progress(used, total int64, width int) string {
	if width <= 0 {
		width = 16
	}
	if total <= 0 {
		return Dim.Render(strings.Repeat("░", width))
	}
	filled := int(float64(used) / float64(total) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return OK.Render(strings.Repeat("█", filled)) + Dim.Render(strings.Repeat("░", width-filled)) + fmt.Sprintf(" %d%%", used*100/total)
}

func Bullet(icon, text string) string {
	return "  " + icon + " " + text
}
