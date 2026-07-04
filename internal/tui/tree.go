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

// buildTree constructs the full config tree for a manifest. Closures capture the
// pointer, so the returned nodes edit m in place.
func buildTree(m *manifest.Manifest) []*node {
	noexec := &node{label: "noexec_onboard", kind: kindReadonly,
		note: "unsupported — would break onboard software",
		getStr: func() string {
			if m.Hardening.Filesystem.NoexecOnboard {
				return "true"
			}
			return "false"
		}}

	plugins := &node{label: "plugins", kind: kindPlugins,
		getList: func() []string { return m.KOReader.Plugins },
		setList: func(v []string) { m.KOReader.Plugins = v }}

	nmNote := &node{label: "entries", kind: kindReadonly,
		note: "edit in koboctl.toml",
		getStr: func() string {
			return strconv.Itoa(len(m.NickelMenu.Entries)) + " entries"
		}}

	roots := []*node{
		group("Device",
			strField("model", &m.Device.Model),
			strField("mount", &m.Device.Mount),
		),
		group("KOReader",
			boolField("enabled", &m.KOReader.Enabled),
			enumField("channel", &m.KOReader.Channel, "stable", "nightly"),
			strField("version", &m.KOReader.Version),
			plugins,
		),
		group("KFMon",
			boolField("enabled", &m.KFMon.Enabled),
		),
		group("NickelMenu",
			boolField("enabled", &m.NickelMenu.Enabled),
			strField("version", &m.NickelMenu.Version),
			nmNote,
		),
		group("Plato",
			boolField("enabled", &m.Plato.Enabled),
			strField("version", &m.Plato.Version),
		),
		group("Hardening",
			boolField("enabled", &m.Hardening.Enabled),
			group("Network",
				enumField("mode", &m.Hardening.Network.Mode, "metadata-only", "offline", "open"),
				listField("dns_servers", &m.Hardening.Network.DNSServers),
				boolField("block_telemetry", &m.Hardening.Network.BlockTelemetry),
				boolField("block_ota", &m.Hardening.Network.BlockOTA),
				boolField("block_sync", &m.Hardening.Network.BlockSync),
			),
			group("Parental",
				boolField("enabled", &m.Hardening.Parental.Enabled),
				boolField("lock_store", &m.Hardening.Parental.LockStore),
				boolField("lock_browser", &m.Hardening.Parental.LockBrowser),
			),
			group("Services",
				boolField("disable_telnet", &m.Hardening.Services.DisableTelnet),
				boolField("disable_ftp", &m.Hardening.Services.DisableFTP),
				boolField("disable_ssh", &m.Hardening.Services.DisableSSH),
			),
			group("Filesystem",
				noexec,
				boolField("disable_koboroot", &m.Hardening.Filesystem.DisableKoboRoot),
				boolField("remove_dangerous_plugins", &m.Hardening.Filesystem.RemoveDangerousPlugins),
			),
			group("Privacy",
				boolField("block_analytics_db", &m.Hardening.Privacy.BlockAnalyticsDB),
				boolField("hosts_blocklist", &m.Hardening.Privacy.HostsBlocklist),
			),
		),
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
