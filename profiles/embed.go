// Package profiles provides embedded device profile TOML files.
package profiles

import "embed"

// FS contains all *.toml device profile files embedded at build time.
//
//go:embed *.toml
var FS embed.FS
