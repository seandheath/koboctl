package tui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seandheath/koboctl/internal/device"
	"github.com/seandheath/koboctl/internal/hardening"
	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

// pollInterval is how often the TUI re-scans for the connected device.
const pollInterval = 2 * time.Second

// compStatus is the install state of a single component.
type compStatus struct {
	name      string
	installed bool
	version   string
}

// deviceStatusMsg carries a device probe result to the model.
type deviceStatusMsg struct {
	di    *device.DeviceInfo
	err   error
	comps []compStatus
	hard  *hardening.HardeningState
}

// tickMsg drives the periodic re-poll.
type tickMsg struct{}

// tickCmd schedules the next poll.
func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// pollDeviceCmd probes the connected Kobo and its installed components. It reads
// only the FAT32 mount (stat/read), so it is cheap enough to run every couple of
// seconds. mountPath overrides auto-detection when non-empty.
func pollDeviceCmd(mountPath string, m *manifest.Manifest) tea.Cmd {
	// Snapshot the hardening config so the goroutine does not race model edits.
	hardCfg := m.Hardening
	hardEnabled := m.Hardening.Enabled
	return func() tea.Msg {
		var di *device.DeviceInfo
		var err error
		if mountPath != "" {
			di, err = device.DetectDevice(mountPath)
		} else {
			di, err = device.AutoDetect()
		}
		if err != nil {
			return deviceStatusMsg{err: err}
		}

		mp := di.MountPoint
		comps := []compStatus{
			probeComponent(mp, "KFMon", installer.IsKFMonInstalled, func() string {
				v, _ := installer.KFMonVersion(mp)
				return v
			}),
			probeComponent(mp, "KOReader", installer.IsKOReaderInstalled, func() string {
				return koreaderRev(mp)
			}),
			probeComponent(mp, "NickelMenu", installer.IsNickelMenuInstalled, nil),
		}

		msg := deviceStatusMsg{di: di, comps: comps}
		if hardEnabled {
			st := hardening.HardeningStatus(mp, hardCfg)
			msg.hard = &st
		}
		return msg
	}
}

// probeComponent runs an installer "IsInstalled" predicate and optional version
// reader into a compStatus. Version is omitted when not installed or "unknown".
func probeComponent(mp, name string, isInstalled func(string) (bool, error), version func() string) compStatus {
	ok, _ := isInstalled(mp)
	cs := compStatus{name: name, installed: ok}
	if ok && version != nil {
		if v := version(); v != "" && v != "unknown" {
			cs.version = v
		}
	}
	return cs
}

// koreaderRev reads the KOReader git revision marker, truncated to 12 chars.
// Mirrors cmd/status.go's koreaderVersion helper (kept local to avoid coupling).
func koreaderRev(mountPath string) string {
	data, err := os.ReadFile(filepath.Join(mountPath, ".adds", "koreader", "git-rev"))
	if err != nil {
		return ""
	}
	rev := strings.TrimSpace(string(data))
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return rev
}
