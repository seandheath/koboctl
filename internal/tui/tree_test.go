package tui

import (
	"testing"

	initcmd "github.com/seandheath/koboctl/internal/init"
)

// findField returns the first field node with the given label under any group.
func findField(roots []*node, label string) *node {
	var found *node
	var walk func([]*node)
	walk = func(ns []*node) {
		for _, n := range ns {
			if n.kind != kindGroup && n.label == label {
				if found == nil {
					found = n
				}
			}
			walk(n.children)
		}
	}
	walk(roots)
	return found
}

func TestBuildTree_BoolToggleMutatesManifest(t *testing.T) {
	m := initcmd.SecureDefaults()
	roots := buildTree(&m)

	// KOReader.enabled is true in SecureDefaults; toggle the first "enabled".
	kore := roots[1] // KOReader group
	if kore.label != "KOReader" {
		t.Fatalf("expected KOReader group, got %q", kore.label)
	}
	en := findField([]*node{kore}, "enabled")
	if en == nil {
		t.Fatal("no enabled field under KOReader")
	}
	before := m.KOReader.Enabled
	en.toggle()
	if m.KOReader.Enabled == before {
		t.Error("toggle did not mutate manifest")
	}
}

func TestBuildTree_EnumCycle(t *testing.T) {
	m := initcmd.SecureDefaults()
	roots := buildTree(&m)
	ch := findField(roots, "channel")
	if ch == nil || ch.kind != kindEnum {
		t.Fatal("channel enum not found")
	}
	m.KOReader.Channel = "stable"
	ch.cycle(1)
	if m.KOReader.Channel != "nightly" {
		t.Errorf("cycle: got %q want nightly", m.KOReader.Channel)
	}
	ch.cycle(1)
	if m.KOReader.Channel != "stable" {
		t.Errorf("cycle wrap: got %q want stable", m.KOReader.Channel)
	}
	ch.cycle(-1)
	if m.KOReader.Channel != "nightly" {
		t.Errorf("cycle back: got %q want nightly", m.KOReader.Channel)
	}
}

func TestFlatten_RespectsCollapse(t *testing.T) {
	m := initcmd.SecureDefaults()
	roots := buildTree(&m)
	full := len(flatten(roots))

	roots[0].expanded = false // collapse Device
	collapsed := len(flatten(roots))
	if collapsed >= full {
		t.Errorf("collapsing Device should hide children: full=%d collapsed=%d", full, collapsed)
	}
}

func TestParseCSV(t *testing.T) {
	got := parseCSV(" 1.1.1.1 , , 8.8.8.8 ,")
	if len(got) != 2 || got[0] != "1.1.1.1" || got[1] != "8.8.8.8" {
		t.Errorf("parseCSV = %v", got)
	}
	if len(parseCSV("")) != 0 {
		t.Error("empty string should yield empty list")
	}
}
