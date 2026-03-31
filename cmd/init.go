package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	initcmd "github.com/seandheath/koboctl/internal/init"
	"github.com/seandheath/koboctl/internal/manifest"
	"github.com/seandheath/koboctl/internal/prompt"
)

func newInitCommand() *cobra.Command {
	var (
		output   string
		defaults bool
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a koboctl.toml manifest interactively",
		Long: `Generate a koboctl.toml manifest by answering a series of questions.

The generated file includes inline comments explaining every option so it is
easy to edit by hand afterwards.

Flags:
  --defaults  Write a secure hardened baseline without prompting.
  --force     Overwrite an existing file.
  -o/--output Output file path (default: koboctl.toml).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Guard against accidental overwrite.
			if _, err := os.Stat(output); err == nil && !force {
				return fmt.Errorf("%q already exists — use --force to overwrite", output)
			}

			var m manifest.Manifest
			if defaults {
				m = initcmd.SecureDefaults()
			} else {
				p := prompt.NewPrompter(cmd.InOrStdin(), cmd.OutOrStdout())
				var err error
				m, err = runQuestionFlow(cmd, p)
				if err != nil {
					return err
				}
			}

			// Validate — catches bugs in the question flow.
			if errs := manifest.ValidateManifest(&m); len(errs) != 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "internal: generated manifest invalid: %v\n", e)
				}
				return fmt.Errorf("manifest validation failed (%d error(s)); this is a bug", len(errs))
			}

			out, err := initcmd.Render(m)
			if err != nil {
				return fmt.Errorf("rendering manifest: %w", err)
			}

			if err := os.WriteFile(output, []byte(out), 0o644); err != nil {
				return fmt.Errorf("writing %q: %w", output, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", output)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: koboctl provision --manifest %s\n", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "koboctl.toml", "output file path")
	cmd.Flags().BoolVar(&defaults, "defaults", false, "write secure defaults without prompting")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	return cmd
}

// runQuestionFlow prompts the user for all manifest options and returns the resulting manifest.
func runQuestionFlow(cmd *cobra.Command, p *prompt.Prompter) (manifest.Manifest, error) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "=== koboctl init ===")
	fmt.Fprintln(out, "Answer a few questions to generate your manifest.")
	fmt.Fprintln(out, "(Press Enter to accept the default shown in brackets.)")
	fmt.Fprintln(out)

	var m manifest.Manifest

	// --- Device ---
	fmt.Fprintln(out, "[Device]")
	model, err := p.String("Device model", "libra-colour")
	if err != nil {
		return m, err
	}
	m.Device.Model = model
	fmt.Fprintln(out)

	// --- KOReader ---
	fmt.Fprintln(out, "[KOReader]")
	koreaderEnabled, err := p.Bool("Install KOReader?", true)
	if err != nil {
		return m, err
	}
	m.KOReader.Enabled = koreaderEnabled

	if koreaderEnabled {
		channel, err := p.Choice("Channel", []string{"stable", "nightly"}, "stable")
		if err != nil {
			return m, err
		}
		m.KOReader.Channel = channel

		version, err := p.String("Version (\"latest\" or e.g. v2024.11)", "latest")
		if err != nil {
			return m, err
		}
		m.KOReader.Version = version

		// KFMon is a required dependency for KOReader — auto-enable.
		fmt.Fprintf(out, "  KFMon will be enabled automatically (required by KOReader).\n")
		m.KFMon.Enabled = true
		// KFMon version is embedded in the binary
	} else {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "[KFMon]")
		kfmonEnabled, err := p.Bool("Install KFMon independently?", false)
		if err != nil {
			return m, err
		}
		m.KFMon.Enabled = kfmonEnabled
		if kfmonEnabled {
			// KFMon version is embedded in the binary
		}
	}
	fmt.Fprintln(out)

	// --- NickelMenu ---
	fmt.Fprintln(out, "[NickelMenu]")
	nmEnabled, err := p.Bool("Install NickelMenu (adds a custom menu to Kobo UI)?", true)
	if err != nil {
		return m, err
	}
	m.NickelMenu.Enabled = nmEnabled
	if nmEnabled {
		m.NickelMenu.Version = "latest"
		if koreaderEnabled {
			fmt.Fprintf(out, "  KOReader launch entry added automatically.\n")
			m.NickelMenu.Entries = initcmd.DefaultNickelMenuEntries(koreaderEnabled)
		}
	}
	fmt.Fprintln(out)

	// --- Plato ---
	fmt.Fprintln(out, "[Plato]")
	platoEnabled, err := p.Bool("Install Plato (alternative reader)?", false)
	if err != nil {
		return m, err
	}
	m.Plato.Enabled = platoEnabled
	fmt.Fprintln(out)

	// --- Hardening ---
	fmt.Fprintln(out, "[Hardening]")
	hardeningEnabled, err := p.Bool("Enable security hardening?", true)
	if err != nil {
		return m, err
	}
	m.Hardening.Enabled = hardeningEnabled

	if hardeningEnabled {
		childSafe, err := p.Bool("Use child-safe defaults for all hardening options?", false)
		if err != nil {
			return m, err
		}
		if childSafe {
			m.Hardening = initcmd.ChildSafeDefaults().Hardening
		} else {
			if err := promptHardeningConfig(p, out, &m.Hardening); err != nil {
				return m, err
			}
		}
	}

	return m, nil
}

// promptHardeningConfig asks all individual hardening questions.
func promptHardeningConfig(p *prompt.Prompter, out io.Writer, h *manifest.HardeningConfig) error {
	h.Enabled = true

	mode, err := p.Choice("Network mode", []string{"metadata-only", "offline", "open"}, "metadata-only")
	if err != nil {
		return err
	}
	h.Network.Mode = mode

	if mode != "offline" {
		dns, err := p.StringList("DNS servers (comma-separated)", []string{"185.228.168.168", "185.228.169.168"})
		if err != nil {
			return err
		}
		h.Network.DNSServers = dns
	}

	blockTelemetry, err := p.Bool("Block Kobo telemetry?", true)
	if err != nil {
		return err
	}
	h.Network.BlockTelemetry = blockTelemetry

	blockOTA, err := p.Bool("Block OTA firmware updates?", true)
	if err != nil {
		return err
	}
	h.Network.BlockOTA = blockOTA

	blockSync, err := p.Bool("Block cloud sync?", true)
	if err != nil {
		return err
	}
	h.Network.BlockSync = blockSync

	parentalEnabled, err := p.Bool("Enable parental controls guidance?", true)
	if err != nil {
		return err
	}
	h.Parental.Enabled = parentalEnabled
	if parentalEnabled {
		lockStore, err := p.Bool("Lock Kobo Store?", true)
		if err != nil {
			return err
		}
		h.Parental.LockStore = lockStore

		lockBrowser, err := p.Bool("Lock web browser?", true)
		if err != nil {
			return err
		}
		h.Parental.LockBrowser = lockBrowser

		fmt.Fprintf(out, "  Note: PIN must be set on-device after provisioning:\n")
		fmt.Fprintf(out, "  More -> Settings -> Accounts -> Parental Controls\n")
	}

	disableTelnet, err := p.Bool("Disable telnet (devmode)?", true)
	if err != nil {
		return err
	}
	h.Services.DisableTelnet = disableTelnet

	disableFTP, err := p.Bool("Disable FTP?", true)
	if err != nil {
		return err
	}
	h.Services.DisableFTP = disableFTP

	disableSSH, err := p.Bool("Disable SSH?", true)
	if err != nil {
		return err
	}
	h.Services.DisableSSH = disableSSH

	guardKoboRoot, err := p.Bool("Guard KoboRoot.tgz (prevents rogue firmware extraction)?", true)
	if err != nil {
		return err
	}
	h.Filesystem.DisableKoboRoot = guardKoboRoot

	removePlugins, err := p.Bool("Remove dangerous KOReader plugins (SSH/WebDAV/browser)?", true)
	if err != nil {
		return err
	}
	h.Filesystem.RemoveDangerousPlugins = removePlugins

	blockAnalytics, err := p.Bool("Block analytics database?", true)
	if err != nil {
		return err
	}
	h.Privacy.BlockAnalyticsDB = blockAnalytics

	hostsBlocklist, err := p.Bool("Enable hosts-file telemetry blocklist?", true)
	if err != nil {
		return err
	}
	h.Privacy.HostsBlocklist = hostsBlocklist

	return nil
}
