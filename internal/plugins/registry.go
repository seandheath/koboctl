// Package plugins holds the built-in registry of installable KOReader plugins.
//
// It has no internal dependencies so that both the manifest validator and the
// installer can import it without creating an import cycle (installer already
// imports manifest).
package plugins

import (
	"sort"
	"strings"
)

// Source describes where a KOReader plugin's release archive comes from.
type Source struct {
	// Owner and Repo identify the GitHub repository publishing releases.
	Owner string
	Repo  string
	// AssetPattern is a path.Match glob selecting the release asset (a zip whose
	// root entry is the "<name>.koplugin/" directory).
	AssetPattern string
}

// registry maps a short plugin name to its release source. Add an entry here to
// make a new plugin installable via `plugins = ["<name>"]` in koboctl.toml.
var registry = map[string]Source{
	// Dynamic Panel Zoom — panel-by-panel navigation for comics/manga.
	// https://github.com/JorgeTheFox/koreader-dynamic-panelzoom
	"dynamic_panelzoom": {
		Owner:        "JorgeTheFox",
		Repo:         "koreader-dynamic-panelzoom",
		AssetPattern: "dynamic_panelzoom.koplugin.zip",
	},
}

// Lookup returns the Source for a registered plugin name.
func Lookup(name string) (Source, bool) {
	s, ok := registry[name]
	return s, ok
}

// Names returns the sorted list of known plugin names (for help text/errors).
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Parse splits a manifest plugin entry "name" or "name@version" into its parts.
// version is "" when no pin is given.
func Parse(entry string) (name, version string) {
	if i := strings.IndexByte(entry, '@'); i >= 0 {
		return entry[:i], entry[i+1:]
	}
	return entry, ""
}
