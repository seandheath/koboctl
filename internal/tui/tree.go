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
		desc:    "Optional KOReader plugins. Press enter to browse and toggle available plugins (e.g. dynamic_panelzoom, scrawl).",
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
			boolField("enabled", &m.Hardening.Enabled).withDesc("Master switch for all hardening below. When off, no hardening is applied."),
			group("Network",
				enumField("mode", &m.Hardening.Network.Mode, "metadata-only", "offline", "open").withDesc("Network policy. 'open' allows all traffic; 'metadata-only' permits only store metadata/sync; 'offline' blocks all networking."),
				listField("dns_servers", &m.Hardening.Network.DNSServers).withDesc("Override the DNS servers the device uses. Leave empty to keep device defaults."),
				boolField("block_telemetry", &m.Hardening.Network.BlockTelemetry).withDesc("Block known Kobo/Rakuten telemetry and analytics endpoints."),
			).withDesc("Controls the device's outbound network behavior."),
			group("Services",
				boolField("disable_telnet", &m.Hardening.Services.DisableTelnet).withDesc("Disable the telnet daemon (an unauthenticated root shell on some firmwares). Recommended."),
				boolField("disable_ftp", &m.Hardening.Services.DisableFTP).withDesc("Disable the FTP server to close an unauthenticated file-transfer surface."),
			).withDesc("Enable or disable on-device network services."),
			group("Filesystem",
				noexec,
				boolField("disable_koboroot", &m.Hardening.Filesystem.DisableKoboRoot).withDesc("Prevent Nickel from auto-applying KoboRoot.tgz packages on boot, blocking a common unsigned-code path."),
			).withDesc("Filesystem-level hardening options."),
			group("Privacy",
				boolField("hosts_blocklist", &m.Hardening.Privacy.HostsBlocklist).withDesc("Install an /etc/hosts blocklist that null-routes tracking and ad domains."),
			).withDesc("Privacy-related mitigations."),
		).withDesc("Security hardening: reduce the device's network exposure and lock down services and firmware-update paths."),
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
