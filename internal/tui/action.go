package tui

import (
	"bufio"
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seandheath/koboctl/internal/backup"
	"github.com/seandheath/koboctl/internal/device"
	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

// Actions injects the high-level orchestration entry points that live in package
// cmd (RunProvision, RunHarden). They are passed in rather than imported to avoid
// an import cycle (cmd imports this package for the `tui` command).
type Actions struct {
	Provision func(mount string, m *manifest.Manifest, dryRun bool) error
	Harden    func(mount string, cfg manifest.HardeningConfig, dryRun, skipGuard bool) error
}

// logLineMsg is a single captured output line from a running action.
type logLineMsg string

// actionDoneMsg signals an action finished (err nil on success).
type actionDoneMsg struct {
	name string
	err  error
}

// runAction runs fn while capturing everything written to os.Stdout/os.Stderr and
// streaming it, line by line, to the Bubble Tea program as logLineMsg. On
// completion it restores the std streams and sends actionDoneMsg.
//
// The installers and hardening steps write progress via fmt.Printf /
// fmt.Fprintf(os.Stderr, …); redirecting the globals lets the TUI show that live
// output without changing any installer signatures. Only one action runs at a
// time (the model's busy guard), so mutating the globals is safe. Bubble Tea's
// renderer captured its own output handle at program creation, so it keeps
// drawing to the real terminal.
func runAction(p *tea.Program, name string, fn func() error) {
	origOut, origErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		p.Send(actionDoneMsg{name: name, err: err})
		return
	}
	os.Stdout, os.Stderr = w, w

	drained := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			p.Send(logLineMsg(sc.Text()))
		}
		close(drained)
	}()

	go func() {
		runErr := fn()
		// Restore streams first, then close the writer so the reader sees EOF.
		os.Stdout, os.Stderr = origOut, origErr
		w.Close()
		<-drained
		r.Close()
		p.Send(actionDoneMsg{name: name, err: runErr})
	}()
}

// installComponentFn returns an action func that installs a single component
// against the given mount using a fresh GitHub client.
func installComponentFn(component, mount string, m *manifest.Manifest) func() error {
	return func() error {
		ctx := context.Background()
		gh := fetch.NewGitHubClient()
		switch component {
		case "kfmon":
			return installer.InstallKFMon(ctx, mount, m.KFMon, gh)
		case "koreader":
			if err := installer.InstallKOReader(ctx, mount, m.KOReader, gh); err != nil {
				return err
			}
			return installer.InstallKOReaderPlugins(ctx, mount, m.KOReader, gh)
		case "nickelmenu":
			return installer.InstallNickelMenu(ctx, mount, m.NickelMenu, gh)
		}
		return nil
	}
}

// backupFn returns an action func that backs up the connected device.
func backupFn(di *device.DeviceInfo) func() error {
	return func() error {
		_, err := backup.CreateBackup(di, backup.BackupOptions{})
		return err
	}
}
