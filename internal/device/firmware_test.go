package device

import (
	"testing"
)

func TestParseVersion_SpecFormat(t *testing.T) {
	// The spec describes this format: affiliate-serial-firmware-modelid
	info, err := ParseVersion("kobo-N123456789-4.39.22801-N428")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}
	if info.Affiliate != "kobo" {
		t.Errorf("Affiliate = %q, want %q", info.Affiliate, "kobo")
	}
	if info.SerialNumber != "N123456789" {
		t.Errorf("SerialNumber = %q, want %q", info.SerialNumber, "N123456789")
	}
	if info.FirmwareVersion != "4.39.22801" {
		t.Errorf("FirmwareVersion = %q, want %q", info.FirmwareVersion, "4.39.22801")
	}
	if info.ModelID != "N428" {
		t.Errorf("ModelID = %q, want %q", info.ModelID, "N428")
	}
}

func TestParseVersion_FallbackOnShortString(t *testing.T) {
	// A bare firmware version string (not the spec format) should not error.
	info, err := ParseVersion("4.39.22801")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}
	// FirmwareVersion should be set to the raw string.
	if info.FirmwareVersion == "" {
		t.Error("FirmwareVersion should not be empty for bare version string")
	}
}

func TestParseVersion_Empty(t *testing.T) {
	_, err := ParseVersion("")
	if err == nil {
		t.Error("expected error for empty version string")
	}
}

func TestParseVersion_Whitespace(t *testing.T) {
	// Trailing newline should be stripped.
	info, err := ParseVersion("kobo-N123456789-4.39.22801-N428\n")
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}
	if info.ModelID != "N428" {
		t.Errorf("ModelID = %q, want %q (whitespace not trimmed?)", info.ModelID, "N428")
	}
}
