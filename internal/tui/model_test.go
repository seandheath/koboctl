package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() *model {
	// Empty path → LoadManifest fails → SecureDefaults fallback (deterministic).
	return newModel("", "", Actions{})
}

func TestModel_WindowSizeSetsLayout(t *testing.T) {
	m := newTestModel()
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = nm.(*model)
	if m.width != 100 || m.height != 40 {
		t.Fatalf("size not stored: %dx%d", m.width, m.height)
	}
	if m.log.Height == 0 {
		t.Error("log viewport height not laid out")
	}
}

func TestModel_ActivateTogglesBool(t *testing.T) {
	m := newTestModel()
	// Move cursor to KOReader's "enabled" field: it's index 4
	// (Device, model, mount, KOReader group, enabled).
	m.cursor = 4
	if m.flat[m.cursor].label != "enabled" {
		t.Fatalf("cursor not on enabled, on %q", m.flat[m.cursor].label)
	}
	before := m.m.KOReader.Enabled
	m.activate()
	if m.m.KOReader.Enabled == before {
		t.Error("activate did not toggle KOReader.enabled")
	}
}

func TestModel_ActivateCollapsesGroup(t *testing.T) {
	m := newTestModel()
	m.cursor = 0 // Device group
	full := len(m.flat)
	m.activate()
	if len(m.flat) >= full {
		t.Errorf("collapsing group did not shrink flat list: %d -> %d", full, len(m.flat))
	}
}

func TestModel_SaveGatesOnValidation(t *testing.T) {
	m := newTestModel()
	// Make the manifest invalid: KOReader enabled but KFMon disabled.
	m.m.KOReader.Enabled = true
	m.m.KFMon.Enabled = false
	m.openSaveDiff()
	if m.modal != modalMessage {
		t.Fatalf("expected validation error modal, got modal=%d", m.modal)
	}
}

func TestModel_GuardedActionNoDevice(t *testing.T) {
	m := newTestModel()
	m.status.di = nil
	cmd := m.guardedAction("provision", func() error { return nil })
	if cmd != nil {
		t.Error("expected nil cmd when no device")
	}
	if m.modal != modalMessage {
		t.Errorf("expected 'no device' message modal, got %d", m.modal)
	}
	if m.busy {
		t.Error("should not be busy without a device")
	}
}

func TestModel_ViewDoesNotPanic(t *testing.T) {
	m := newTestModel()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m.View() // main layout

	// With a simulated connected device + status.
	di := deviceStatusMsg{}
	m.status = di
	_ = m.View()

	// Each modal render path.
	for _, mk := range []modalKind{modalMessage, modalConfirm, modalDiff, modalInstall, modalPlugins} {
		m.modal = mk
		m.modalTitle = "t"
		m.modalLines = []string{"+ a", "- b", "ctx"}
		m.choices = []string{"x", "y"}
		m.pluginChecks = map[string]bool{"x": true}
		_ = m.View()
	}
}

func TestModel_ViewDescReflectsCursor(t *testing.T) {
	m := newTestModel()
	// Cursor index 4 is KOReader's "enabled" field (see TestModel_ActivateTogglesBool).
	m.cursor = 4
	n := m.flat[m.cursor]
	if n.label != "enabled" || n.desc == "" {
		t.Fatalf("unexpected node under cursor: label=%q desc=%q", n.label, n.desc)
	}
	out := m.viewDesc()
	if !strings.Contains(out, n.label) {
		t.Errorf("viewDesc missing label %q: %q", n.label, out)
	}
	if !strings.Contains(out, n.desc) {
		t.Errorf("viewDesc missing description %q: %q", n.desc, out)
	}
}

func TestModel_ApplyPluginsPreservesPins(t *testing.T) {
	m := newTestModel()
	m.m.KOReader.Plugins = []string{"dynamic_panelzoom@v1.7.0"}
	m.choices = []string{"dynamic_panelzoom"}
	m.pluginChecks = map[string]bool{"dynamic_panelzoom": true}
	m.applyPlugins()
	if len(m.m.KOReader.Plugins) != 1 || m.m.KOReader.Plugins[0] != "dynamic_panelzoom@v1.7.0" {
		t.Errorf("pin not preserved: %v", m.m.KOReader.Plugins)
	}
	// Unchecking removes it.
	m.pluginChecks["dynamic_panelzoom"] = false
	m.applyPlugins()
	if len(m.m.KOReader.Plugins) != 0 {
		t.Errorf("unchecked plugin not removed: %v", m.m.KOReader.Plugins)
	}
}
