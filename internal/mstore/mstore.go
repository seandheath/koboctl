// Package mstore resolves where the koboctl manifest lives and reads/writes it.
//
// koboctl is device-primary: when a Kobo is connected, its
// .adds/koboctl/koboctl.toml is the source of truth. The host path (the
// --manifest flag) is a fallback used only when no device is connected, or when
// a connected device has no config yet.
package mstore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seandheath/koboctl/internal/device"
	initcmd "github.com/seandheath/koboctl/internal/init"
	"github.com/seandheath/koboctl/internal/manifest"
)

// deviceRel is the manifest location on the device, under koboctl's own dir.
const deviceRel = ".adds/koboctl/koboctl.toml"

// DevicePath returns the on-device manifest path for a mount point.
func DevicePath(mount string) string {
	return filepath.Join(mount, ".adds", "koboctl", "koboctl.toml")
}

// Detect returns the connected Kobo, honoring an explicit mount override. It
// never errors: a nil result simply means no device is available.
func Detect(mountPath string) *device.DeviceInfo {
	var di *device.DeviceInfo
	var err error
	if mountPath != "" {
		di, err = device.DetectDevice(mountPath)
	} else {
		di, err = device.AutoDetect()
	}
	if err != nil {
		return nil
	}
	return di
}

// Resolved describes where the active manifest was loaded from.
type Resolved struct {
	Manifest *manifest.Manifest
	Path     string             // path the manifest was loaded from
	Device   *device.DeviceInfo // connected device, or nil
	OnDevice bool               // true when Path is the device copy
}

// Load implements device-primary resolution. If a device is connected and has a
// manifest at DevicePath, that is loaded; otherwise hostPath is loaded. The
// detected device is returned either way so callers can persist to it.
func Load(hostPath, mountPath string) (*Resolved, error) {
	di := Detect(mountPath)
	if di != nil {
		dp := DevicePath(di.MountPoint)
		if fileExists(dp) {
			m, err := manifest.LoadManifest(dp)
			if err != nil {
				return nil, fmt.Errorf("loading device manifest %q: %w", dp, err)
			}
			return &Resolved{Manifest: m, Path: dp, Device: di, OnDevice: true}, nil
		}
	}
	m, err := manifest.LoadManifest(hostPath)
	if err != nil {
		return nil, err
	}
	return &Resolved{Manifest: m, Path: hostPath, Device: di}, nil
}

// Save writes m to the device when one is connected, otherwise to hostPath.
// Returns the path written. Device-primary: a connected Kobo always wins.
func Save(m *manifest.Manifest, hostPath string, di *device.DeviceInfo) (string, error) {
	if di != nil {
		return WriteToDevice(di.MountPoint, m)
	}
	if err := writeRendered(hostPath, m); err != nil {
		return "", err
	}
	return hostPath, nil
}

// WriteToDevice renders m to DevicePath(mount), creating .adds/koboctl/ as needed.
func WriteToDevice(mount string, m *manifest.Manifest) (string, error) {
	dp := DevicePath(mount)
	if err := writeRendered(dp, m); err != nil {
		return "", err
	}
	return dp, nil
}

// writeRendered serializes m via the annotated template and writes it to path.
func writeRendered(path string, m *manifest.Manifest) error {
	out, err := initcmd.Render(*m)
	if err != nil {
		return fmt.Errorf("rendering manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating dir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
