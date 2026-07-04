package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the interactive TUI against the given manifest path and optional
// mount override. actions injects the provision/harden orchestration from
// package cmd (passed in to avoid an import cycle).
func Run(manifestPath, mountPath string, actions Actions) error {
	m := newModel(manifestPath, mountPath, actions)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.prog = p // background action goroutines Send through this
	_, err := p.Run()
	return err
}
