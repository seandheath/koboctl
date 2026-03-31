// Package config generates configuration files for Kobo software components.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/manifest"
)

// GenerateNickelMenuConfig generates NickelMenu DSL text from the given entries.
//
// Output format per entry:
//
//	menu_item :location :label :action :arg
//	chain_success :chain-action :chain-arg   (only if Chain is set)
//
// The Chain field encodes "action:arg..." — split on the first colon only,
// since the arg portion may itself contain colons (e.g., "quiet:/path/to/script").
func GenerateNickelMenuConfig(entries []manifest.NickelMenuEntry) string {
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "menu_item :%s :%s :%s :%s\n", e.Location, e.Label, e.Action, e.Arg)
		if e.Chain != "" {
			// Split "action:arg..." on the first colon only.
			parts := strings.SplitN(e.Chain, ":", 2)
			chainAction := parts[0]
			chainArg := ""
			if len(parts) == 2 {
				chainArg = parts[1]
			}
			fmt.Fprintf(&sb, "chain_success :%s :%s\n", chainAction, chainArg)
		}
	}
	return sb.String()
}

// WriteNickelMenuConfig writes the generated NickelMenu config to
// <mountPath>/.adds/nm/config, creating parent directories as needed.
func WriteNickelMenuConfig(mountPath string, entries []manifest.NickelMenuEntry) error {
	content := GenerateNickelMenuConfig(entries)
	dest := filepath.Join(mountPath, ".adds", "nm", "config")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating NickelMenu config directory: %w", err)
	}
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing NickelMenu config to %q: %w", dest, err)
	}
	return nil
}
