package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seandheath/koboctl/internal/manifest"
)

// goldenManifest is a representative koboctl.toml fixture.
const goldenManifest = `
[device]
model = "libra-colour"
mount = ""

[koreader]
enabled = true
channel = "stable"
version = "latest"

[nickelmenu]
enabled = true
version = "latest"

[[nickelmenu.entries]]
location = "main"
label = "KOReader"
action = "dbg_toast"
arg = "Starting KOReader..."
chain = "cmd_spawn:quiet:/usr/bin/kfmon-ipc trigger koreader"

[kfmon]
enabled = true
version = "latest"

[hardening]
enabled = true

[hardening.network]
mode = "metadata-only"
dns_servers = ["185.228.168.168", "185.228.169.168"]
block_telemetry = true
block_ota = true
block_sync = true

[hardening.parental]
enabled = true
lock_store = true
lock_browser = true

[hardening.services]
disable_telnet = true
disable_ftp = true
disable_ssh = true

[hardening.filesystem]
noexec_onboard = false
disable_koboroot = true
remove_dangerous_plugins = true

[hardening.privacy]
block_analytics_db = true
hosts_blocklist = true
`

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "koboctl.toml")
	if err := os.WriteFile(path, []byte(goldenManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := manifest.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Device.Model != "libra-colour" {
		t.Errorf("Device.Model = %q, want %q", m.Device.Model, "libra-colour")
	}
	if !m.KOReader.Enabled {
		t.Error("KOReader.Enabled should be true")
	}
	if m.KOReader.Channel != "stable" {
		t.Errorf("KOReader.Channel = %q, want %q", m.KOReader.Channel, "stable")
	}
	if !m.KFMon.Enabled {
		t.Error("KFMon.Enabled should be true")
	}
	if len(m.NickelMenu.Entries) != 1 {
		t.Errorf("NickelMenu.Entries length = %d, want 1", len(m.NickelMenu.Entries))
	} else {
		e := m.NickelMenu.Entries[0]
		if e.Label != "KOReader" {
			t.Errorf("NickelMenu.Entries[0].Label = %q, want %q", e.Label, "KOReader")
		}
		if e.Chain != "cmd_spawn:quiet:/usr/bin/kfmon-ipc trigger koreader" {
			t.Errorf("NickelMenu.Entries[0].Chain = %q (unexpected)", e.Chain)
		}
	}
	if !m.Hardening.Enabled {
		t.Error("Hardening.Enabled should be true")
	}
	if m.Hardening.Network.Mode != "metadata-only" {
		t.Errorf("Hardening.Network.Mode = %q, want %q", m.Hardening.Network.Mode, "metadata-only")
	}
	if len(m.Hardening.Network.DNSServers) != 2 {
		t.Errorf("Hardening.Network.DNSServers length = %d, want 2", len(m.Hardening.Network.DNSServers))
	}
	if !m.Hardening.Privacy.HostsBlocklist {
		t.Error("Hardening.Privacy.HostsBlocklist should be true")
	}
}

func TestValidateManifest_KFMonRequired(t *testing.T) {
	m := &manifest.Manifest{}
	m.KOReader.Enabled = true
	m.KFMon.Enabled = false

	errs := manifest.ValidateManifest(m)
	if len(errs) == 0 {
		t.Error("expected validation error when koreader enabled but kfmon disabled")
	}
}

func TestValidateManifest_InvalidChannel(t *testing.T) {
	m := &manifest.Manifest{}
	m.KOReader.Enabled = true
	m.KOReader.Channel = "beta"
	m.KFMon.Enabled = true

	errs := manifest.ValidateManifest(m)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid channel")
	}
}

func TestValidateManifest_MissingEntryLabel(t *testing.T) {
	m := &manifest.Manifest{}
	m.NickelMenu.Entries = []manifest.NickelMenuEntry{
		{Location: "main", Label: ""},
	}

	errs := manifest.ValidateManifest(m)
	if len(errs) == 0 {
		t.Error("expected validation error for entry with empty label")
	}
}

func TestValidateManifest_Valid(t *testing.T) {
	m := &manifest.Manifest{}
	m.KOReader.Enabled = true
	m.KOReader.Channel = "stable"
	m.KOReader.Version = "v2024.11"
	m.KFMon.Enabled = true
	m.NickelMenu.Entries = []manifest.NickelMenuEntry{
		{Location: "main", Label: "KOReader", Action: "dbg_toast", Arg: "Starting..."},
	}

	errs := manifest.ValidateManifest(m)
	if len(errs) != 0 {
		t.Errorf("unexpected validation errors: %v", errs)
	}
}

func TestValidateManifest_InvalidHardeningMode(t *testing.T) {
	m := &manifest.Manifest{}
	m.Hardening.Enabled = true
	m.Hardening.Network.Mode = "paranoid"

	errs := manifest.ValidateManifest(m)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid hardening network mode")
	}
}

func TestValidateManifest_HardeningDNSRequired(t *testing.T) {
	m := &manifest.Manifest{}
	m.Hardening.Enabled = true
	m.Hardening.Network.Mode = "metadata-only"
	m.Hardening.Network.BlockTelemetry = true
	// DNSServers intentionally empty

	errs := manifest.ValidateManifest(m)
	if len(errs) == 0 {
		t.Error("expected validation error when dns_servers empty and block_telemetry true")
	}
}
