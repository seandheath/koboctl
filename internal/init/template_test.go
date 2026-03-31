package initcmd_test

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	initcmd "github.com/seandheath/koboctl/internal/init"
	"github.com/seandheath/koboctl/internal/manifest"
)

func TestRender_RoundTrip(t *testing.T) {
	m := initcmd.SecureDefaults()

	out, err := initcmd.Render(m)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var parsed manifest.Manifest
	if err := toml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("generated TOML is not parseable: %v\n---\n%s", err, out)
	}

	// Spot-check key fields survive the round trip.
	if parsed.Device.Model != m.Device.Model {
		t.Errorf("Device.Model = %q, want %q", parsed.Device.Model, m.Device.Model)
	}
	if parsed.KOReader.Enabled != m.KOReader.Enabled {
		t.Errorf("KOReader.Enabled = %v, want %v", parsed.KOReader.Enabled, m.KOReader.Enabled)
	}
	if parsed.KOReader.Channel != m.KOReader.Channel {
		t.Errorf("KOReader.Channel = %q, want %q", parsed.KOReader.Channel, m.KOReader.Channel)
	}
	if parsed.KFMon.Enabled != m.KFMon.Enabled {
		t.Errorf("KFMon.Enabled = %v, want %v", parsed.KFMon.Enabled, m.KFMon.Enabled)
	}
	if parsed.NickelMenu.Enabled != m.NickelMenu.Enabled {
		t.Errorf("NickelMenu.Enabled = %v, want %v", parsed.NickelMenu.Enabled, m.NickelMenu.Enabled)
	}
	if len(parsed.NickelMenu.Entries) != len(m.NickelMenu.Entries) {
		t.Errorf("NickelMenu.Entries len = %d, want %d", len(parsed.NickelMenu.Entries), len(m.NickelMenu.Entries))
	}
	if !parsed.Hardening.Enabled {
		t.Error("Hardening.Enabled should be true")
	}
	if parsed.Hardening.Network.Mode != m.Hardening.Network.Mode {
		t.Errorf("Hardening.Network.Mode = %q, want %q", parsed.Hardening.Network.Mode, m.Hardening.Network.Mode)
	}
	if len(parsed.Hardening.Network.DNSServers) != len(m.Hardening.Network.DNSServers) {
		t.Errorf("DNSServers len = %d, want %d", len(parsed.Hardening.Network.DNSServers), len(m.Hardening.Network.DNSServers))
	}
	if !parsed.Hardening.Filesystem.DisableKoboRoot {
		t.Error("Hardening.Filesystem.DisableKoboRoot should be true")
	}
	if parsed.Hardening.Filesystem.NoexecOnboard {
		t.Error("noexec_onboard must always be false")
	}
}

func TestRender_CommentsPresent(t *testing.T) {
	out, err := initcmd.Render(initcmd.SecureDefaults())
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"metadata-only",
		"CleanBrowsing",
		"NOT SUPPORTED",
		"Parental controls PIN must be set manually",
		"Additional [[nickelmenu.entries]] blocks can be added manually",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Errorf("expected comment text %q in rendered output", want)
		}
	}
}

func TestRender_ChildSafeDefaults_RoundTrip(t *testing.T) {
	m := initcmd.ChildSafeDefaults()

	out, err := initcmd.Render(m)
	if err != nil {
		t.Fatalf("Render(ChildSafeDefaults): %v", err)
	}

	var parsed manifest.Manifest
	if err := toml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("ChildSafeDefaults TOML unparseable: %v\n---\n%s", err, out)
	}

	if parsed.Hardening.Network.Mode != "offline" {
		t.Errorf("ChildSafeDefaults: mode = %q, want offline", parsed.Hardening.Network.Mode)
	}
}

func TestRender_EmptyNickelMenuEntries(t *testing.T) {
	m := initcmd.SecureDefaults()
	m.NickelMenu.Entries = nil

	out, err := initcmd.Render(m)
	if err != nil {
		t.Fatalf("Render with no entries: %v", err)
	}

	var parsed manifest.Manifest
	if err := toml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("TOML with no entries unparseable: %v\n---\n%s", err, out)
	}
	if len(parsed.NickelMenu.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(parsed.NickelMenu.Entries))
	}
}
