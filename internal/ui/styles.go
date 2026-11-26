package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5A3FD9", Dark: "#A78BFA"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colGreen  = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}
	colRed    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colYellow = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	mutedStyle   = lipgloss.NewStyle().Foreground(colMuted)
	successStyle = lipgloss.NewStyle().Foreground(colGreen)
	errorStyle   = lipgloss.NewStyle().Foreground(colRed)
	warnStyle    = lipgloss.NewStyle().Foreground(colYellow).Bold(true)
	sectionStyle = lipgloss.NewStyle().Bold(true).MarginTop(1)
	selectedRow  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	promptStyle  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	commandStyle = lipgloss.NewStyle().Bold(true)
	commandBox   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1)
	dangerBox = commandBox.BorderForeground(colRed)
	keyStyle  = lipgloss.NewStyle().Foreground(colAccent)
)

// helpLine renders "key desc · key desc" pairs.
func helpLine(pairs ...string) string {
	s := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			s += mutedStyle.Render(" · ")
		}
		s += keyStyle.Render(pairs[i]) + " " + mutedStyle.Render(pairs[i+1])
	}
	return s
}

// HumanSize formats bytes as e.g. "1.9 GB".
func HumanSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
