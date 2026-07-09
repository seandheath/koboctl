package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKFMonKOReaderINI_BootFlag verifies the on_boot keys are present only when
// booting into KOReader is requested, and that they sit inside the [kobo] watch
// section (before [kobo-target]).
func TestKFMonKOReaderINI_BootFlag(t *testing.T) {
	off := kfmonKOReaderINI(false)
	if strings.Contains(off, "on_boot") {
		t.Errorf("boot disabled: unexpected on_boot key:\n%s", off)
	}

	on := kfmonKOReaderINI(true)
	for _, key := range []string{"on_boot = true", "on_boot_trigger = true"} {
		if !strings.Contains(on, key) {
			t.Errorf("boot enabled: missing %q:\n%s", key, on)
		}
	}
	// on_boot must be within the [kobo] section, i.e. before [kobo-target].
	if strings.Index(on, "on_boot = true") > strings.Index(on, "[kobo-target]") {
		t.Errorf("on_boot key is not inside the [kobo] section:\n%s", on)
	}
}

// TestWriteKFMonKOReaderConfig writes the config to a temp mount and checks the
// resulting koreader.ini reflects the boot flag.
func TestWriteKFMonKOReaderConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteKFMonKOReaderConfig(dir, true); err != nil {
		t.Fatalf("WriteKFMonKOReaderConfig: %v", err)
	}

	dest := filepath.Join(dir, ".adds", "kfmon", "config", "koreader.ini")
	if ok, _ := CheckInstalled(dest); !ok {
		t.Fatalf("koreader.ini not written to %s", dest)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "on_boot = true") {
		t.Errorf("koreader.ini missing on_boot when boot requested:\n%s", data)
	}
}
