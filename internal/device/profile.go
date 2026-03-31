// Package device handles Kobo device detection and model profiles.
package device

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/seandheath/koboctl/profiles"
)

// DeviceProfile describes the hardware characteristics of a supported Kobo model.
type DeviceProfile struct {
	// ModelID is the Kobo model identifier (e.g., "N428" for Libra Colour).
	ModelID string `toml:"model_id"`
	// Name is the human-readable model name.
	Name string `toml:"name"`
	// Architecture is the target ABI (e.g., "arm-kobo-linux-gnueabihf").
	Architecture string `toml:"architecture"`
}

// profileFile wraps the top-level TOML structure which nests under [device].
type profileFile struct {
	Device DeviceProfile `toml:"device"`
}

// GetProfile looks up a device profile by model ID (e.g., "N428").
// Returns an error if the model is not found in the embedded profile database.
func GetProfile(modelID string) (*DeviceProfile, error) {
	all, err := ListProfiles()
	if err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	for i := range all {
		if strings.EqualFold(all[i].ModelID, modelID) {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("unknown device model %q — not in profile database", modelID)
}

// ListProfiles returns all device profiles embedded in the binary.
func ListProfiles() ([]DeviceProfile, error) {
	entries, err := fs.ReadDir(profiles.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("reading profiles FS: %w", err)
	}

	var out []DeviceProfile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		data, err := profiles.FS.ReadFile(e.Name())
		if err != nil {
			return nil, fmt.Errorf("reading profile %q: %w", e.Name(), err)
		}
		var pf profileFile
		if err := toml.Unmarshal(data, &pf); err != nil {
			return nil, fmt.Errorf("parsing profile %q: %w", e.Name(), err)
		}
		out = append(out, pf.Device)
	}
	return out, nil
}
