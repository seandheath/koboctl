package tui

import "github.com/charmbracelet/lipgloss"

// styles holds the lipgloss styles used across the TUI. Colors are ANSI-256 so
// they degrade gracefully in limited terminals (the Kobo workflow is often run
// over SSH/tmux).
type styles struct {
	title       lipgloss.Style
	paneFocused lipgloss.Style
	paneBlurred lipgloss.Style
	group       lipgloss.Style
	fieldName   lipgloss.Style
	fieldVal    lipgloss.Style
	cursor      lipgloss.Style
	ok          lipgloss.Style
	bad         lipgloss.Style
	dim         lipgloss.Style
	actionBar   lipgloss.Style
	modal       lipgloss.Style
	diffAdd     lipgloss.Style
	diffDel     lipgloss.Style
}

// accent colors
var (
	colAccent = lipgloss.Color("39") // blue
	colGreen  = lipgloss.Color("42")
	colRed    = lipgloss.Color("203")
	colDim    = lipgloss.Color("242")
	colYellow = lipgloss.Color("214")
)

func newStyles() styles {
	return styles{
		title:       lipgloss.NewStyle().Bold(true).Foreground(colAccent),
		paneFocused: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colAccent).Padding(0, 1),
		paneBlurred: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colDim).Padding(0, 1),
		group:       lipgloss.NewStyle().Bold(true),
		fieldName:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		fieldVal:    lipgloss.NewStyle().Foreground(colAccent),
		cursor:      lipgloss.NewStyle().Foreground(colYellow).Bold(true),
		ok:          lipgloss.NewStyle().Foreground(colGreen),
		bad:         lipgloss.NewStyle().Foreground(colRed),
		dim:         lipgloss.NewStyle().Foreground(colDim),
		actionBar:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		modal:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colYellow).Padding(1, 2),
		diffAdd:     lipgloss.NewStyle().Foreground(colGreen),
		diffDel:     lipgloss.NewStyle().Foreground(colRed),
	}
}
