package device

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FirmwareInfo contains parsed device identity from /.kobo/version (and fallbacks).
type FirmwareInfo struct {
	Affiliate       string
	SerialNumber    string
	FirmwareVersion string
	ModelID         string
	// Raw is the raw content of /.kobo/version.
	Raw string
}

// ParseVersion parses the colon-delimited version string from /.kobo/version.
//
// The spec describes the format as:
//
//	<affiliate>-<serialnumber>-<firmwareversion>-<modelid>
//
// For example: "kobo-N123456789-4.39.22801-N428"
//
// Note: actual Kobo firmware may store a different format. If parsing fails or
// the model ID is missing, callers should fall back to ReadVersionFiles.
func ParseVersion(content string) (*FirmwareInfo, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("version file is empty")
	}

	// Try the spec format: affiliate-serial-firmware-modelid (dash-delimited, 4 parts).
	// We split on "-" but firmware versions like "4.39.22801" also contain dashes,
	// so we split into exactly 4 parts from the right.
	// Format: <affiliate>-<serial>-<firmware>-<modelid>
	// Since affiliate and serial don't contain dashes, split into at most 4 parts.
	parts := strings.SplitN(content, "-", 4)
	if len(parts) == 4 {
		info := &FirmwareInfo{
			Affiliate:       parts[0],
			SerialNumber:    parts[1],
			FirmwareVersion: parts[2],
			ModelID:         parts[3],
			Raw:             content,
		}
		if info.ModelID != "" && info.FirmwareVersion != "" {
			return info, nil
		}
	}

	// Fall back: treat the whole string as a firmware version, model unknown.
	// The caller should use ReadVersionFiles for the full picture.
	return &FirmwareInfo{
		FirmwareVersion: content,
		Raw:             content,
	}, nil
}

// ReadVersionFiles reads device identity from the individual files that some
// Kobo firmware versions use instead of (or in addition to) /.kobo/version.
//
//   - /.kobo/version  — may contain just the firmware version string
//   - /.kobo/.model   — model ID (e.g., "N428")
//   - /.kobo/serial   — serial number
//
// This is the fallback when ParseVersion cannot extract a model ID.
func ReadVersionFiles(mountPath string) (*FirmwareInfo, error) {
	koboDir := filepath.Join(mountPath, ".kobo")

	info := &FirmwareInfo{}

	// /.kobo/version — firmware version string
	if data, err := os.ReadFile(filepath.Join(koboDir, "version")); err == nil {
		info.Raw = strings.TrimSpace(string(data))
		parsed, _ := ParseVersion(info.Raw)
		if parsed != nil {
			*info = *parsed
		}
	}

	// /.kobo/.model — model ID (overrides anything parsed from version)
	if data, err := os.ReadFile(filepath.Join(koboDir, ".model")); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			info.ModelID = id
		}
	}

	// /.kobo/serial — serial number (overrides anything parsed from version)
	if data, err := os.ReadFile(filepath.Join(koboDir, "serial")); err == nil {
		if sn := strings.TrimSpace(string(data)); sn != "" {
			info.SerialNumber = sn
		}
	}

	if info.FirmwareVersion == "" && info.ModelID == "" {
		return nil, fmt.Errorf("could not determine firmware version or model ID from %q", koboDir)
	}

	return info, nil
}
