# koboctl

Provision and harden hacked Kobo e-readers from a Linux workstation, driven by a
declarative `koboctl.toml` manifest. Installs KOReader (+ plugins), KFMon, and
NickelMenu, then applies a child-safe security hardening pass.

## Interactive TUI

Running `koboctl` with no arguments on a terminal opens the interactive TUI
(`koboctl tui` does the same):

```
┌ koboctl ● ──────────────────────────┬ Device ────────────────┐
│ ▾ KOReader                          │ ● connected            │
│   enabled            ✓              │ Kobo Libra Colour (N428)│
│   channel            stable         │ fw 4.39.22801          │
│   version            latest         │ /media/u/KOBOeReader   │
│   plugins            1 selected     │ ─ components ─         │
│ ▸ KFMon  ▸ NickelMenu               │ ✓ KFMon    v1.4.6      │
│ ▾ Hardening                         │ ✓ KOReader v2024.11    │
│   ▸ Network ▸ Parental ▸ Services   │ ✗ NickelMenu           │
├──────────────────────────────────────┴───────────────────────┤
│ [P]rovision  [I]nstall  [H]arden  [B]ackup  [S]ave  [Q]uit    │
├────────────────────────────────────────────────────────────────┤
│ log ▸ kfmon: extracting embedded v1.4.6…                      │
└────────────────────────────────────────────────────────────────┘
```

- **Live device panel** — auto-detects a plugged-in Kobo every ~2s: model,
  firmware, mount, installed components + versions, and hardening state.
- **Full manifest editor** — every option grouped/collapsible. `space` toggles
  bools, `←/→` or `enter` cycles enums, `enter` edits text fields, the plugin
  browser is a checklist.
- **Run actions with live output** — provision/install/harden/backup stream their
  output into the log pane. Provision/harden show a **dry-run preview** first and
  ask before applying.
- **Save with diff** — `S` validates, shows a diff of what will change, and writes
  `koboctl.toml`.

Keys: `↑/↓` move · `space/enter` toggle/edit · `tab` switch pane · `ctrl+r`
refresh · `q` quit.

### Where the config lives

koboctl is **device-primary**: when a Kobo is connected, the manifest lives on
the device at `.adds/koboctl/koboctl.toml` and is the source of truth. The host
`--manifest` file (default `koboctl.toml`) is a fallback used only when no device
is connected, or when a connected device has no config yet. `init` mirrors the
generated config onto the device, `provision` persists it there, and the TUI's
Save writes to the device — so the config travels with the reader and any
workstation manages the same copy.

Existing subcommands still run headless for scripting:

```
koboctl init                 # generate koboctl.toml
koboctl status [--json]      # report device + component state
koboctl provision [--dry-run]
koboctl install <koreader|kfmon|nickelmenu>
koboctl harden [--dry-run]
koboctl backup [-o file] / koboctl restore <file>
```

## Build

```
nix develop        # dev shell with the full toolchain
make build         # -> bin/koboctl (CGO-free static binary)
make test
make run           # launch the TUI
```

See `docs/log.md` for design decisions and rationale.
