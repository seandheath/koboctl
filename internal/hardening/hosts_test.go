package hardening_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seandheath/koboctl/internal/hardening"
	"github.com/seandheath/koboctl/internal/manifest"
)

// metadataDomains lists domains that KOReader needs for metadata fetching.
// These must NOT appear in the blocklist.
var metadataDomains = []string{
	"openlibrary.org",
	"covers.openlibrary.org",
	"www.googleapis.com",
	"books.google.com",
}

func TestBlocklistEntries_ContainsTelemetryDomains(t *testing.T) {
	entries := hardening.BlocklistEntries()
	if len(entries) == 0 {
		t.Fatal("BlocklistEntries returned empty list")
	}

	required := []string{
		"storeapi.kobo.com",
		"telemetry.kobo.com",
		"www.google-analytics.com",
		"script.hotjar.com",
		"stats.g.doubleclick.net",
	}
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		set[e] = true
	}
	for _, want := range required {
		if !set[want] {
			t.Errorf("BlocklistEntries missing required domain %q", want)
		}
	}
}

func TestBlocklistEntries_DoesNotBlockMetadata(t *testing.T) {
	entries := hardening.BlocklistEntries()
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		set[e] = true
	}
	for _, domain := range metadataDomains {
		if set[domain] {
			t.Errorf("BlocklistEntries must not block KOReader metadata domain %q", domain)
		}
	}
}

func TestStageHostsBlocklist_WritesScript(t *testing.T) {
	dir := t.TempDir()
	cfg := manifest.HardeningConfig{}
	cfg.Privacy.HostsBlocklist = true

	if err := hardening.StageHostsBlocklist(dir, cfg); err != nil {
		t.Fatalf("StageHostsBlocklist: %v", err)
	}

	scriptPath := filepath.Join(dir, ".adds", "koboctl", "harden-hosts.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("script not written: %v", err)
	}

	content := string(data)
	// Verify script contains the marker and a sample domain.
	if !strings.Contains(content, "koboctl-blocklist-start") {
		t.Error("script missing idempotency marker")
	}
	if !strings.Contains(content, "0.0.0.0 telemetry.kobo.com") {
		t.Error("script missing telemetry blocklist entry")
	}
	// Verify metadata domains are NOT blocked.
	for _, domain := range metadataDomains {
		if strings.Contains(content, "0.0.0.0 "+domain) {
			t.Errorf("script must not block metadata domain %q", domain)
		}
	}
}

func TestStageHostsBlocklist_SkipsWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := manifest.HardeningConfig{}
	cfg.Privacy.HostsBlocklist = false

	if err := hardening.StageHostsBlocklist(dir, cfg); err != nil {
		t.Fatalf("StageHostsBlocklist: %v", err)
	}

	scriptPath := filepath.Join(dir, ".adds", "koboctl", "harden-hosts.sh")
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Error("script should not be written when hosts_blocklist is false")
	}
}

func TestStageHostsBlocklist_Idempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := manifest.HardeningConfig{}
	cfg.Privacy.HostsBlocklist = true

	if err := hardening.StageHostsBlocklist(dir, cfg); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := hardening.StageHostsBlocklist(dir, cfg); err != nil {
		t.Fatalf("second call: %v", err)
	}
}
