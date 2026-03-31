package manifest

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

// semverLoose matches version strings like "v2024.11" or "v1.4.2".
var semverLoose = regexp.MustCompile(`^v\d+(\.\d+)+$`)

// LoadManifest reads and parses a TOML manifest from the given file path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %q: %w", path, err)
	}
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %q: %w", path, err)
	}
	return &m, nil
}

// ValidateManifest checks for required fields and logical consistency.
// Returns all validation errors found (not just the first).
func ValidateManifest(m *Manifest) []error {
	var errs []error

	add := func(err error) { errs = append(errs, err) }

	// KFMon must be enabled if KOReader is enabled (KFMon is a launch dependency).
	if m.KOReader.Enabled && !m.KFMon.Enabled {
		add(errors.New("kfmon must be enabled when koreader is enabled (KFMon is a dependency)"))
	}

	// Validate KOReader channel.
	if m.KOReader.Enabled {
		ch := m.KOReader.Channel
		if ch != "" && ch != "stable" && ch != "nightly" {
			add(fmt.Errorf("koreader.channel must be \"stable\" or \"nightly\", got %q", ch))
		}
		ver := m.KOReader.Version
		if ver != "" && ver != "latest" && !semverLoose.MatchString(ver) {
			add(fmt.Errorf("koreader.version must be \"latest\" or a version tag (e.g., v2024.11), got %q", ver))
		}
	}

	// Validate NickelMenu entries.
	for i, e := range m.NickelMenu.Entries {
		if e.Location == "" {
			add(fmt.Errorf("nickelmenu.entries[%d]: location is required", i))
		}
		if e.Label == "" {
			add(fmt.Errorf("nickelmenu.entries[%d]: label is required", i))
		}
	}

	// Validate hardening config.
	if m.Hardening.Enabled {
		mode := m.Hardening.Network.Mode
		if mode != "" && mode != "metadata-only" && mode != "offline" && mode != "open" {
			add(fmt.Errorf("hardening.network.mode must be \"metadata-only\", \"offline\", or \"open\", got %q", mode))
		}
		// DNS servers required unless mode is offline (no network).
		if mode != "offline" && m.Hardening.Network.BlockTelemetry {
			if len(m.Hardening.Network.DNSServers) == 0 {
				add(errors.New("hardening.network.dns_servers is required when block_telemetry is true"))
			}
		}
	}

	return errs
}
