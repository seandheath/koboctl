package tui

import (
	"strconv"
	"strings"

	"github.com/seandheath/koboctl/internal/manifest"
)

// nodeKind classifies a config-tree row.
type nodeKind int

const (
	kindGroup      nodeKind = iota // collapsible header
	kindBool                       // space toggles
	kindEnum                       // enter/←→ cycles a fixed value set
	kindString                     // enter opens a text editor
	kindStringList                 // enter opens a CSV text editor
	kindPlugins                    // enter opens the plugin browser modal
	kindReadonly                   // shown but not editable
)

// node is a single row in the config tree. Groups own children; fields bind
// getter/setter closures over the live *manifest.Manifest so edits mutate it in
// place. Only the closures relevant to the kind are set.
type node struct {
	label    string
	kind     nodeKind
	depth    int
	note     string // trailing hint, e.g. why a field is read-only
	desc     string // one-paragraph explanation shown in the description panel
	expanded bool
	children []*node

	// field accessors (set per kind)
	getBool func() bool
	setBool func(bool)

	enumVals []string
	getStr   func() string // also used for kindString display/value
	setStr   func(string)

	getList func() []string
	setList func([]string)
}

// value renders the current field value for display.
func (n *node) value() string {
	switch n.kind {
	case kindBool:
		if n.getBool() {
			return "✓"
		}
		return "✗"
	case kindEnum, kindString, kindReadonly:
		v := ""
		if n.getStr != nil {
			v = n.getStr()
		}
		if v == "" {
			return "—"
		}
		return v
	case kindStringList:
		l := n.getList()
		if len(l) == 0 {
			return "—"
		}
		return strings.Join(l, ", ")
	case kindPlugins:
		l := n.getList()
		if len(l) == 0 {
			return "none"
		}
		return strconv.Itoa(len(l)) + " selected"
	}
	return ""
}

// toggle flips a bool field.
func (n *node) toggle() {
	if n.kind == kindBool && n.setBool != nil {
		n.setBool(!n.getBool())
	}
}

// cycle advances an enum field by dir (+1/-1), wrapping.
func (n *node) cycle(dir int) {
	if n.kind != kindEnum || len(n.enumVals) == 0 {
		return
	}
	cur := n.getStr()
	idx := 0
	for i, v := range n.enumVals {
		if v == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(n.enumVals)) % len(n.enumVals)
	n.setStr(n.enumVals[idx])
}

// boolField builds a bool node bound to a *bool.
func boolField(label string, p *bool) *node {
	return &node{label: label, kind: kindBool,
		getBool: func() bool { return *p },
		setBool: func(v bool) { *p = v }}
}

// enumField builds an enum node bound to a *string with a fixed value set.
func enumField(label string, p *string, vals ...string) *node {
	return &node{label: label, kind: kindEnum, enumVals: vals,
		getStr: func() string { return *p },
		setStr: func(v string) { *p = v }}
}

// strField builds a free-text node bound to a *string.
func strField(label string, p *string) *node {
	return &node{label: label, kind: kindString,
		getStr: func() string { return *p },
		setStr: func(v string) { *p = v }}
}

// listField builds a CSV-editable list node bound to a *[]string.
func listField(label string, p *[]string) *node {
	return &node{label: label, kind: kindStringList,
		getList: func() []string { return *p },
		setList: func(v []string) { *p = v }}
}

// group builds a collapsible header, expanded by default.
func group(label string, children ...*node) *node {
	return &node{label: label, kind: kindGroup, expanded: true, children: children}
}

// withDesc attaches a description shown in the TUI's description panel. Returns
// the node so it can be chained onto a builder.
func (n *node) withDesc(s string) *node { n.desc = s; return n }

// buildTree constructs the full config tree for a manifest. Closures capture the
// pointer, so the returned nodes edit m in place.
func buildTree(m *manifest.Manifest) []*node {
	noexec := &node{label: "noexec_onboard", kind: kindReadonly,
		note: "unsupported — would break onboard software",
		desc: "Would mount the onboard partition noexec. Unsupported — it breaks onboard software, so it stays off.",
		getStr: func() string {
			if m.Hardening.Filesystem.NoexecOnboard {
				return "true"
			}
			return "false"
		}}

	plugins := &node{label: "plugins", kind: kindPlugins,
		desc:    "Optional KOReader plugins. Press enter to browse and toggle available plugins (e.g. dynamic_panelzoom).",
		getList: func() []string { return m.KOReader.Plugins },
		setList: func(v []string) { m.KOReader.Plugins = v }}

	nmNote := &node{label: "entries", kind: kindReadonly,
		note: "edit in koboctl.toml",
		desc: "Custom menu entries. Edit these directly in koboctl.toml.",
		getStr: func() string {
			return strconv.Itoa(len(m.NickelMenu.Entries)) + " entries"
		}}

	roots := []*node{
		group("Device",
			strField("model", &m.Device.Model).withDesc("The Kobo hardware model (e.g. Clara HD, Libra 2). Selects the firmware and binaries koboctl targets. Usually auto-detected from the connected device."),
			strField("mount", &m.Device.Mount).withDesc("Filesystem mount point of the device's user partition. Leave blank to auto-detect the connected Kobo."),
		).withDesc("Physical Kobo device. Auto-detected when connected over USB."),
		group("KOReader",
			boolField("enabled", &m.KOReader.Enabled).withDesc("Install and manage KOReader on the device, and add a launcher so it starts from the home screen."),
			strField("version", &m.KOReader.Version).withDesc("Pin a specific KOReader release tag. Leave blank to track the latest known-good release."),
			boolField("boot_into_koreader", &m.KOReader.BootIntoKOReader).withDesc("Launch KOReader automatically when the device powers on, instead of the stock Kobo home screen (Nickel). Uses KFMon's on-boot hook; exiting KOReader returns to Nickel. Requires KOReader enabled."),
			plugins,
		).withDesc("A full-featured document reader (PDF, EPUB, DjVu, CBZ) with advanced typesetting and a plugin system."),
		group("KFMon",
			boolField("enabled", &m.KFMon.Enabled).withDesc("Install KFMon, the on-device launcher that makes third-party apps startable directly from Nickel."),
		).withDesc("A launcher/watchdog that starts KOReader, Plato, or other apps from the Kobo home screen via 'book' cover icons — no USB needed."),
		group("NickelMenu",
			boolField("enabled", &m.NickelMenu.Enabled).withDesc("Install NickelMenu to add custom entries to the Kobo's native menus."),
			strField("version", &m.NickelMenu.Version).withDesc("Pin a specific NickelMenu release. Leave blank for the latest known-good release."),
			nmNote,
		).withDesc("Injects custom entries into the stock Nickel reader UI for quick access to scripts and apps."),
		group("Plato",
			boolField("enabled", &m.Plato.Enabled).withDesc("Install and manage the Plato reader on the device."),
			strField("version", &m.Plato.Version).withDesc("Pin a specific Plato release. Leave blank for the latest known-good release."),
		).withDesc("A lightweight alternative document reader focused on speed and a minimal interface."),
		group("Hardening",
			boolField("enabled", &m.Hardening.Enabled).withDesc("Master switch for the hardening below. When on, koboctl always: edits .kobo/Kobo/Kobo eReader.conf (AutoUpdateEnabled=false, AutoSync=false, EnableDebugServices=false, SideloadedMode=true); adds an AFTER INSERT trigger on AnalyticsEvents in KoboReader.sqlite that deletes rows as they arrive (DB backed up to KoboReader.sqlite.koboctl-backup); and removes the webbrowser/SSH/MyWebDav/send2ebook .koplugin dirs. Root-filesystem changes can't be written over USB, so they are staged as .adds/koboctl/harden-*.sh scripts run at each boot via the KFMon on_boot hook."),
			group("Network",
				enumField("mode", &m.Hardening.Network.Mode, "metadata-only", "offline", "open").withDesc("Any non-empty value (or block_telemetry) stages harden-dns.sh, which at boot rewrites /etc/resolv.conf to the DNS servers below and marks it immutable (chattr +i). Note: offline/open are not yet distinct — currently any non-empty value simply enables the DNS lockdown."),
				listField("dns_servers", &m.Hardening.Network.DNSServers).withDesc("The nameserver lines harden-dns.sh writes into /etc/resolv.conf (default CleanBrowsing Family Filter 185.228.168.168 / 185.228.169.168). The file is set immutable (chattr +i) so udhcpc can't overwrite it on WiFi connect."),
				boolField("block_telemetry", &m.Hardening.Network.BlockTelemetry).withDesc("Also enables the harden-dns.sh DNS lockdown (uses dns_servers). Separate from the /etc/hosts telemetry blocklist, which is the privacy.hosts_blocklist toggle."),
			).withDesc("Outbound network behavior, applied via boot scripts on the ext4 root filesystem."),
			group("Services",
				boolField("disable_telnet", &m.Hardening.Services.DisableTelnet).withDesc("Stages harden-devmode.sh: each boot runs pkill telnetd and strips telnet from /etc/inetd.conf(.local), then HUPs inetd — closes the passwordless-root telnet backdoor opened by typing 'devmodeon' on the device."),
				boolField("disable_ftp", &m.Hardening.Services.DisableFTP).withDesc("harden-devmode.sh also strips ftp from /etc/inetd.conf and HUPs inetd, closing the FTP file-transfer service."),
			).withDesc("On-device network services, disabled via the harden-devmode.sh boot script."),
			group("Filesystem",
				noexec,
				boolField("disable_koboroot", &m.Hardening.Filesystem.DisableKoboRoot).withDesc("Replaces .kobo/KoboRoot.tgz (the package the firmware auto-extracts as root on boot) with a same-named directory so no rogue update can be applied; harden-koboroot.sh re-establishes the guard after the legitimate first-boot firmware update consumes the original file."),
			).withDesc("Filesystem-level hardening."),
			group("Privacy",
				boolField("hosts_blocklist", &m.Hardening.Privacy.HostsBlocklist).withDesc("Stages harden-hosts.sh: appends 0.0.0.0 entries to /etc/hosts for Kobo/Rakuten telemetry, Google Analytics, DoubleClick, Hotjar, and ipinfodb (idempotent, marker-guarded)."),
			).withDesc("Privacy mitigations."),
		).withDesc("Security hardening: reduce the device's network exposure and lock down services and firmware-update paths. Details on each option below."),
	}
	setDepth(roots, 0)
	return roots
}

// setDepth assigns tree depth recursively for indentation.
func setDepth(nodes []*node, depth int) {
	for _, n := range nodes {
		n.depth = depth
		if len(n.children) > 0 {
			setDepth(n.children, depth+1)
		}
	}
}

// flatten returns the visible rows in display order, honoring collapsed groups.
func flatten(roots []*node) []*node {
	var out []*node
	var walk func(nodes []*node)
	walk = func(nodes []*node) {
		for _, n := range nodes {
			out = append(out, n)
			if n.kind == kindGroup && n.expanded {
				walk(n.children)
			}
		}
	}
	walk(roots)
	return out
}

// parseCSV splits a comma-separated editor value into a trimmed, non-empty list.
func parseCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}
