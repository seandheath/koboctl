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
	fmt.Fprintf(out, "    Used to select the correct firmware and artifact builds.\n")
	fmt.Fprintf(out, "    Currently supported: libra-colour. Other models are untested.\n")
	model, err := p.String("Device model", "libra-colour")
	if err != nil {
		return m, err
	}
	m.Device.Model = model
	fmt.Fprintln(out)

	// --- KOReader ---
	fmt.Fprintln(out, "[KOReader]")
	fmt.Fprintf(out, "    Open-source e-book reader supporting EPUB, PDF, DjVu, and more.\n")
	fmt.Fprintf(out, "    Installed to .adds/koreader/ on the FAT32 partition.\n")
	fmt.Fprintf(out, "    Automatically enables KFMon (filesystem monitor required to launch it).\n")
	koreaderEnabled, err := p.Bool("Install KOReader?", true)
	if err != nil {
		return m, err
	}
	m.KOReader.Enabled = koreaderEnabled

	if koreaderEnabled {
		fmt.Fprintf(out, "    \"stable\" tracks official tagged releases.\n")
		fmt.Fprintf(out, "    \"nightly\" pulls the latest development build (may have bugs).\n")
		channel, err := p.Choice("Channel", []string{"stable", "nightly"}, "stable")
		if err != nil {
			return m, err
		}
		m.KOReader.Channel = channel

		fmt.Fprintf(out, "    Use \"latest\" for the newest release, or pin a tag like \"v2024.11\".\n")
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
		fmt.Fprintf(out, "    KFMon is a filesystem monitor that launches apps from the home screen\n")
		fmt.Fprintf(out, "    via book cover images. Only needed standalone if you want to run other\n")
		fmt.Fprintf(out, "    KFMon-compatible apps without KOReader.\n")
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
	fmt.Fprintf(out, "    Injects custom menu entries into the stock Kobo UI (Nickel).\n")
	fmt.Fprintf(out, "    If KOReader is enabled, a \"KOReader\" launch button is added automatically.\n")
	fmt.Fprintf(out, "    Installed via KoboRoot.tgz extraction on next device reboot.\n")
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
	fmt.Fprintf(out, "    Lightweight alternative document reader (EPUB, PDF, CBZ, DJVU).\n")
	fmt.Fprintf(out, "    WARNING: Not yet supported by koboctl. This option is a placeholder.\n")
	platoEnabled, err := p.Bool("Install Plato (alternative reader)?", false)
	if err != nil {
		return m, err
	}
	m.Plato.Enabled = platoEnabled
	fmt.Fprintln(out)

	// --- Hardening ---
	fmt.Fprintln(out, "[Hardening]")
	fmt.Fprintf(out, "    Master toggle for all security hardening options below.\n")
	fmt.Fprintf(out, "    Covers network restrictions, service disabling, telemetry blocking,\n")
	fmt.Fprintf(out, "    filesystem guards, and privacy protections. Each can be tuned individually.\n")
	hardeningEnabled, err := p.Bool("Enable security hardening?", true)
	if err != nil {
		return m, err
	}
	m.Hardening.Enabled = hardeningEnabled

	if hardeningEnabled {
		fmt.Fprintf(out, "    Applies the most restrictive hardening preset:\n")
		fmt.Fprintf(out, "    - Network mode set to \"offline\" (all outbound traffic blocked)\n")
		fmt.Fprintf(out, "    - All telemetry, OTA updates, and cloud sync blocked\n")
		fmt.Fprintf(out, "    - All remote services (telnet, FTP, SSH) disabled\n")
		fmt.Fprintf(out, "    - KoboRoot guard active, dangerous plugins removed\n")
		fmt.Fprintf(out, "    Skips all individual hardening questions below.\n")
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

	fmt.Fprintf(out, "    Controls what network access the device has:\n")
	fmt.Fprintf(out, "    - metadata-only: WiFi works for book covers and metadata only;\n")
	fmt.Fprintf(out, "      telemetry/sync blocked via hosts file and DNS filtering.\n")
	fmt.Fprintf(out, "    - offline: All outbound network traffic blocked. No WiFi.\n")
	fmt.Fprintf(out, "    - open: No network restrictions applied.\n")
	mode, err := p.Choice("Network mode", []string{"metadata-only", "offline", "open"}, "metadata-only")
	if err != nil {
		return err
	}
	h.Network.Mode = mode

	if mode != "offline" {
		fmt.Fprintf(out, "    Overwrites /etc/resolv.conf and locks it immutable (chattr +i)\n")
		fmt.Fprintf(out, "    so the DHCP client cannot revert it on WiFi connect.\n")
		fmt.Fprintf(out, "    Default: CleanBrowsing Family Filter — blocks adult content,\n")
		fmt.Fprintf(out, "    malware, and phishing at the DNS level.\n")
		dns, err := p.StringList("DNS servers (comma-separated)", []string{"185.228.168.168", "185.228.169.168"})
		if err != nil {
			return err
		}
		h.Network.DNSServers = dns
	}

	fmt.Fprintf(out, "    Installs a SQLite trigger on the AnalyticsEvents table that\n")
	fmt.Fprintf(out, "    auto-deletes telemetry rows on insert. Also blocked at the\n")
	fmt.Fprintf(out, "    network level if the hosts-file blocklist is enabled.\n")
	blockTelemetry, err := p.Bool("Block Kobo telemetry?", true)
	if err != nil {
		return err
	}
	h.Network.BlockTelemetry = blockTelemetry

	fmt.Fprintf(out, "    Sets AutoUpdateEnabled=false in the Kobo config file.\n")
	fmt.Fprintf(out, "    Prevents automatic firmware downloads that could reset hardening.\n")
	blockOTA, err := p.Bool("Block OTA firmware updates?", true)
	if err != nil {
		return err
	}
	h.Network.BlockOTA = blockOTA

	fmt.Fprintf(out, "    Sets AutoSync=false in the Kobo config file.\n")
	fmt.Fprintf(out, "    Reading position and bookmarks stay on-device only.\n")
	blockSync, err := p.Bool("Block cloud sync?", true)
	if err != nil {
		return err
	}
	h.Network.BlockSync = blockSync

	fmt.Fprintf(out, "    Enables options to lock the Kobo Store and web browser.\n")
	fmt.Fprintf(out, "    A 4-digit PIN must be set manually on-device after provisioning\n")
	fmt.Fprintf(out, "    (More -> Settings -> Accounts -> Parental Controls).\n")
	parentalEnabled, err := p.Bool("Enable parental controls guidance?", true)
	if err != nil {
		return err
	}
	h.Parental.Enabled = parentalEnabled
	if parentalEnabled {
		fmt.Fprintf(out, "    Prevents browsing or purchasing from the Kobo Store without the PIN.\n")
		lockStore, err := p.Bool("Lock Kobo Store?", true)
		if err != nil {
			return err
		}
		h.Parental.LockStore = lockStore

		fmt.Fprintf(out, "    Prevents access to the built-in Kobo web browser without the PIN.\n")
		lockBrowser, err := p.Bool("Lock web browser?", true)
		if err != nil {
			return err
		}
		h.Parental.LockBrowser = lockBrowser

		fmt.Fprintf(out, "  Note: PIN must be set on-device after provisioning:\n")
		fmt.Fprintf(out, "  More -> Settings -> Accounts -> Parental Controls\n")
	}

	fmt.Fprintf(out, "    Typing \"devmodeon\" in the Kobo search bar enables telnet on port 23\n")
	fmt.Fprintf(out, "    with root access and no password. This kills telnetd and removes it\n")
	fmt.Fprintf(out, "    from inetd.conf on each boot.\n")
	disableTelnet, err := p.Bool("Disable telnet (devmode)?", true)
	if err != nil {
		return err
	}
	h.Services.DisableTelnet = disableTelnet

	fmt.Fprintf(out, "    Removes the FTP server from inetd.conf, closing file transfer access.\n")
	fmt.Fprintf(out, "    Books can still be transferred via USB mass storage.\n")
	disableFTP, err := p.Bool("Disable FTP?", true)
	if err != nil {
		return err
	}
	h.Services.DisableFTP = disableFTP

	fmt.Fprintf(out, "    Removes SSH from inetd.conf, closing secure shell access.\n")
	fmt.Fprintf(out, "    WARNING: Once disabled, the only way to get shell access is via\n")
	fmt.Fprintf(out, "    serial console or re-provisioning with this option turned off.\n")
	disableSSH, err := p.Bool("Disable SSH?", true)
	if err != nil {
		return err
	}
	h.Services.DisableSSH = disableSSH

	fmt.Fprintf(out, "    Replaces .kobo/KoboRoot.tgz with a directory of the same name.\n")
	fmt.Fprintf(out, "    The firmware init script checks \"-f KoboRoot.tgz\" which fails on a\n")
	fmt.Fprintf(out, "    directory, blocking both rogue and legitimate firmware extraction.\n")
	fmt.Fprintf(out, "    To apply a firmware update later, remove the guard directory first.\n")
	guardKoboRoot, err := p.Bool("Guard KoboRoot.tgz (prevents rogue firmware extraction)?", true)
	if err != nil {
		return err
	}
	h.Filesystem.DisableKoboRoot = guardKoboRoot

	fmt.Fprintf(out, "    Deletes KOReader plugins that expose network services or browsing:\n")
	fmt.Fprintf(out, "    webbrowser, SSH server, WebDAV file server, and send2ebook receiver.\n")
	fmt.Fprintf(out, "    Keeps metadata/cover fetching, OPDS catalogs, and Calibre plugins.\n")
	removePlugins, err := p.Bool("Remove dangerous KOReader plugins (SSH/WebDAV/browser)?", true)
	if err != nil {
		return err
	}
	h.Filesystem.RemoveDangerousPlugins = removePlugins

	fmt.Fprintf(out, "    Creates a SQLite trigger on the AnalyticsEvents table that deletes\n")
	fmt.Fprintf(out, "    all rows after any insert. Prevents telemetry accumulation even if\n")
	fmt.Fprintf(out, "    the hosts-file blocklist is somehow bypassed.\n")
	blockAnalytics, err := p.Bool("Block analytics database?", true)
	if err != nil {
		return err
	}
	h.Privacy.BlockAnalyticsDB = blockAnalytics

	fmt.Fprintf(out, "    Appends 14 domains to /etc/hosts pointing to 0.0.0.0, run at boot.\n")
	fmt.Fprintf(out, "    Blocks: Kobo telemetry/store APIs, Google Analytics, DoubleClick,\n")
	fmt.Fprintf(out, "    Hotjar behavior tracking, and IP geolocation fingerprinting.\n")
	hostsBlocklist, err := p.Bool("Enable hosts-file telemetry blocklist?", true)
	if err != nil {
		return err
	}
	h.Privacy.HostsBlocklist = hostsBlocklist

	return nil
}
