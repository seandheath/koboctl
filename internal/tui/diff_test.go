package tui

import "testing"

func TestLineDiff(t *testing.T) {
	old := "a\nb\nc"
	newT := "a\nx\nc"
	changes := changedOnly(lineDiff(old, newT))
	if len(changes) != 2 {
		t.Fatalf("expected 2 changed lines, got %d: %+v", len(changes), changes)
	}
	// Order: removal then addition at the divergence point.
	if changes[0].kind != '-' || changes[0].text != "b" {
		t.Errorf("first change = %c%q", changes[0].kind, changes[0].text)
	}
	if changes[1].kind != '+' || changes[1].text != "x" {
		t.Errorf("second change = %c%q", changes[1].kind, changes[1].text)
	}
}

func TestLineDiff_NoChange(t *testing.T) {
	if c := changedOnly(lineDiff("same\ntext", "same\ntext")); len(c) != 0 {
		t.Errorf("identical text should have no changes, got %+v", c)
	}
}

func TestLineDiff_Append(t *testing.T) {
	c := changedOnly(lineDiff("a", "a\nb"))
	if len(c) != 1 || c[0].kind != '+' || c[0].text != "b" {
		t.Errorf("append diff = %+v", c)
	}
}
