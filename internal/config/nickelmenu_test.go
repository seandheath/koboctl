package config_test

import (
	"strings"
	"testing"

	"github.com/seandheath/koboctl/internal/config"
	"github.com/seandheath/koboctl/internal/manifest"
)

func TestGenerateNickelMenuConfig_SimpleEntry(t *testing.T) {
	entries := []manifest.NickelMenuEntry{
		{
			Location: "main",
			Label:    "Force WiFi",
			Action:   "nickel_setting",
			Arg:      "enable:force_wifi",
		},
	}
	got := config.GenerateNickelMenuConfig(entries)
	want := "menu_item :main :Force WiFi :nickel_setting :enable:force_wifi\n"
	if got != want {
		t.Errorf("GenerateNickelMenuConfig:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateNickelMenuConfig_ChainEntry(t *testing.T) {
	entries := []manifest.NickelMenuEntry{
		{
			Location: "main",
			Label:    "KOReader",
			Action:   "dbg_toast",
			Arg:      "Starting KOReader...",
			Chain:    "cmd_spawn:quiet:/usr/bin/kfmon-ipc trigger koreader",
		},
	}
	got := config.GenerateNickelMenuConfig(entries)

	// Must contain the menu_item line with action and arg.
	if !strings.Contains(got, "menu_item :main :KOReader :dbg_toast :Starting KOReader...") {
		t.Errorf("missing menu_item line in:\n%s", got)
	}
	// Must contain the chain_success line with the full path (colons in arg preserved).
	if !strings.Contains(got, "chain_success :cmd_spawn :quiet:/usr/bin/kfmon-ipc trigger koreader") {
		t.Errorf("missing or mangled chain_success line in:\n%s", got)
	}
}

func TestGenerateNickelMenuConfig_MultipleEntries(t *testing.T) {
	entries := []manifest.NickelMenuEntry{
		{Location: "main", Label: "A", Action: "act1", Arg: "arg1"},
		{Location: "main", Label: "B", Action: "act2", Arg: "arg2"},
	}
	got := config.GenerateNickelMenuConfig(entries)

	// Two entries should be separated by a blank line.
	if !strings.Contains(got, "\nmenu_item :main :B") {
		t.Errorf("expected blank line between entries:\n%s", got)
	}
}

func TestGenerateNickelMenuConfig_Empty(t *testing.T) {
	got := config.GenerateNickelMenuConfig(nil)
	if got != "" {
		t.Errorf("expected empty string for nil entries, got: %q", got)
	}
}
