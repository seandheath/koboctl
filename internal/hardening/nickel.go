package hardening

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/manifest"
)

// nickelSettings is the set of INI key-value pairs to enforce, grouped by section.
// Keys not present in this map are preserved as-is.
//
// Sections and keys are matched case-insensitively, but output uses the canonical case
// specified here.
var nickelSettings = map[string]map[string]string{
	"FeatureSettings": {
		// Block OTA firmware auto-updates — prevents hardening from being reset.
		"AutoUpdateEnabled": "false",
	},
	"ApplicationPreferences": {
		// No Kobo account required; enables sideloaded content mode.
		"SideloadedMode": "true",
	},
	"DeveloperSettings": {
		// Keep WiFi on for KOReader metadata fetching.
		"EnableWifi": "true",
		// Don't sync library with Kobo cloud.
		"AutoSync": "false",
		// Disable developer debug services (telnet, etc.).
		"EnableDebugServices": "false",
	},
	"Library": {
		// Exclude dot-prefixed directories (except .kobo and .adobe) from Nickel
		// library scanning. Prevents KOReader system files (icons, resources) from
		// appearing as books. Required since firmware 4.17+ scans dot-dirs.
		"ExcludeSyncFolders": `\.(?!kobo|adobe).*`,
	},
}

// HardenNickelConfig applies security settings to .kobo/Kobo/Kobo eReader.conf.
//
// The file is in Windows-style INI format. This function reads the file, applies
// the required key-value pairs, and writes it back. All existing sections and keys
// not managed by this function are preserved, including ordering and whitespace.
//
// This file lives on the FAT32 partition and can be written directly during USB
// provisioning (no boot script required).
func HardenNickelConfig(mountPoint string, cfg manifest.HardeningConfig) error {
	confPath := filepath.Join(mountPoint, ".kobo", "Kobo", "Kobo eReader.conf")

	// Read existing file; start with empty content if it doesn't exist yet.
	data, err := os.ReadFile(confPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading nickel config: %w", err)
	}

	lines := parseINI(string(data))
	lines = applySettings(lines, nickelSettings)

	out := renderINI(lines)
	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return fmt.Errorf("creating nickel config directory: %w", err)
	}
	return os.WriteFile(confPath, []byte(out), 0o644)
}

// iniLine represents a single line in the INI file (section header, key=value, or raw).
type iniLine struct {
	section string // current section (populated for key lines)
	key     string // empty for section headers and raw lines
	value   string
	raw     string // original text (preserved for section headers, comments, blanks)
}

// parseINI reads an INI file into a slice of iniLine structs.
// Section headers and comments are stored as raw lines; key=value pairs are parsed.
func parseINI(content string) []iniLine {
	var lines []iniLine
	currentSection := ""

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed[1 : len(trimmed)-1]
			lines = append(lines, iniLine{raw: text})
			continue
		}

		if idx := strings.IndexByte(trimmed, '='); idx > 0 && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
			k := strings.TrimSpace(trimmed[:idx])
			v := strings.TrimSpace(trimmed[idx+1:])
			lines = append(lines, iniLine{section: currentSection, key: k, value: v})
			continue
		}

		lines = append(lines, iniLine{raw: text})
	}

	return lines
}

// applySettings merges wanted section→key→value pairs into the parsed lines.
// Existing keys are updated in place; missing keys are appended to their section.
// Missing sections are appended at the end.
func applySettings(lines []iniLine, settings map[string]map[string]string) []iniLine {
	// Track which settings have been applied.
	applied := make(map[string]map[string]bool)
	for sec := range settings {
		applied[sec] = make(map[string]bool)
	}

	// Update existing keys.
	for i, l := range lines {
		if l.key == "" {
			continue
		}
		secSettings, ok := settings[l.section]
		if !ok {
			continue
		}
		// Case-insensitive key match.
		for wantKey, wantVal := range secSettings {
			if strings.EqualFold(l.key, wantKey) {
				lines[i].value = wantVal
				lines[i].key = wantKey // normalise to canonical case
				applied[l.section][wantKey] = true
			}
		}
	}

	// Append missing keys to existing sections, then append missing sections.
	for sec, kvs := range settings {
		for k, v := range kvs {
			if applied[sec][k] {
				continue
			}
			// Find the last line in this section and insert after it.
			inserted := false
			for i := len(lines) - 1; i >= 0; i-- {
				if lines[i].section == sec || (lines[i].raw != "" && strings.TrimSpace(lines[i].raw) == "["+sec+"]") {
					// Insert after this position.
					newLine := iniLine{section: sec, key: k, value: v}
					lines = append(lines[:i+1], append([]iniLine{newLine}, lines[i+1:]...)...)
					inserted = true
					break
				}
			}
			if !inserted {
				// Section not found — append section header + key.
				lines = append(lines, iniLine{raw: "[" + sec + "]"})
				lines = append(lines, iniLine{section: sec, key: k, value: v})
			}
		}
	}

	return lines
}

// renderINI converts parsed lines back to INI text.
func renderINI(lines []iniLine) string {
	var sb strings.Builder
	for _, l := range lines {
		if l.key != "" {
			fmt.Fprintf(&sb, "%s=%s\n", l.key, l.value)
		} else {
			sb.WriteString(l.raw)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
