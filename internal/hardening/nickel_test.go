package hardening_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seandheath/koboctl/internal/hardening"
	"github.com/seandheath/koboctl/internal/manifest"
)

// sampleNickelConf is a representative Kobo eReader.conf fixture with existing settings.
const sampleNickelConf = `[Reading]
ReadingGestureEnabled=true
PageTurnAnimation=true

[FeatureSettings]
; OTA was previously enabled
AutoUpdateEnabled=true
SomeOtherKey=keep_me

[ApplicationPreferences]
Theme=dark

[DeveloperSettings]
EnableWifi=false
SomeDevKey=somevalue
`

func TestHardenNickelConfig_AppliesSettings(t *testing.T) {
	dir := t.TempDir()
	koboDir := filepath.Join(dir, ".kobo", "Kobo")
	if err := os.MkdirAll(koboDir, 0o755); err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(koboDir, "Kobo eReader.conf")
	if err := os.WriteFile(confPath, []byte(sampleNickelConf), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := manifest.HardeningConfig{}
	if err := hardening.HardenNickelConfig(dir, cfg); err != nil {
		t.Fatalf("HardenNickelConfig: %v", err)
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)

	// Required hardened values.
	cases := []struct{ key, want string }{
		{"AutoUpdateEnabled", "false"},
		{"SideloadedMode", "true"},
		{"AutoSync", "false"},
		{"EnableDebugServices", "false"},
		{"EnableWifi", "true"},
	}
	for _, tc := range cases {
		if !strings.Contains(out, tc.key+"="+tc.want) {
			t.Errorf("expected %s=%s in output:\n%s", tc.key, tc.want, out)
		}
	}
}

func TestHardenNickelConfig_PreservesUnmanagedKeys(t *testing.T) {
	dir := t.TempDir()
	koboDir := filepath.Join(dir, ".kobo", "Kobo")
	if err := os.MkdirAll(koboDir, 0o755); err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(koboDir, "Kobo eReader.conf")
	if err := os.WriteFile(confPath, []byte(sampleNickelConf), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := manifest.HardeningConfig{}
	if err := hardening.HardenNickelConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)

	// Unmanaged keys from other sections must survive.
	preserved := []string{
		"ReadingGestureEnabled=true",
		"PageTurnAnimation=true",
		"Theme=dark",
		"SomeOtherKey=keep_me",
		"SomeDevKey=somevalue",
	}
	for _, want := range preserved {
		if !strings.Contains(out, want) {
			t.Errorf("expected preserved key %q in output:\n%s", want, out)
		}
	}
}

func TestHardenNickelConfig_CreatesFileIfAbsent(t *testing.T) {
	dir := t.TempDir()
	// Don't create the file — it should be created from scratch.
	cfg := manifest.HardeningConfig{}
	if err := hardening.HardenNickelConfig(dir, cfg); err != nil {
		t.Fatalf("HardenNickelConfig on absent file: %v", err)
	}

	confPath := filepath.Join(dir, ".kobo", "Kobo", "Kobo eReader.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("config not created: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "AutoUpdateEnabled=false") {
		t.Error("new file should contain hardened settings")
	}
}

func TestHardenNickelConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	cfg := manifest.HardeningConfig{}
	if err := hardening.HardenNickelConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	data1, _ := os.ReadFile(filepath.Join(dir, ".kobo", "Kobo", "Kobo eReader.conf"))

	if err := hardening.HardenNickelConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, ".kobo", "Kobo", "Kobo eReader.conf"))

	if string(data1) != string(data2) {
		t.Error("second run should produce identical output (idempotent)")
	}
}
