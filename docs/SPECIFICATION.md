# koboctl — Kobo E-Reader Configuration CLI

## Overview

`koboctl` is a Go CLI tool for provisioning and hardening Kobo e-readers from a Linux workstation over USB. It automates installation of KOReader, NickelMenu, KFMon, and Plato, and applies a comprehensive security hardening profile — turning a stock Kobo into a locked-down, vendor-free reading device in a single command.

The tool operates in **USB mode**: filesystem operations on a Kobo mounted as USB Mass Storage. All modifications are staged to the FAT32 partition (`/mnt/onboard`) accessible via USB; modifications to the ext4 root filesystem (e.g., `/etc/hosts`) are staged as boot scripts executed at first boot via the KFMon `on_boot` hook.

## Goals

- **One-command provisioning**: `koboctl provision` takes a stock Kobo from box to fully configured in under five minutes.
- **Idempotent operations**: Every command can be run repeatedly without breaking state. The tool detects what is already installed and skips or upgrades as needed.
- **Reproducible configuration**: All configuration is declarative via a TOML manifest file. The same manifest rebuilds the same device state.
- **Offline-capable**: All required artifacts can be pre-cached locally. The tool never requires the Kobo to have internet access during provisioning.
- **Security by default**: Hardening is enabled in default manifests. Telemetry is blocked, OTA updates are disabled, parental controls are guided.
- **Extensible by model**: Primary target is the Kobo Libra Colour (MT8113T, armhf, firmware 4.x). Architecture supports adding other Kobo models via a device profile system.

## Non-Goals

- Bypassing Secure Boot or flashing custom kernels.
- Managing ebook content or library sync (Calibre handles this via USB or Calibre wireless device driver).
- Providing a GUI or TUI — this is a CLI tool.
- Supporting non-Kobo e-readers.
- Network management, SSH access, or Tailscale (removed from scope).

## Target Device Profile: Kobo Libra Colour

| Property          | Value                                      |
|-------------------|--------------------------------------------|
| Model ID          | `N428`                                     |
| SoC               | MediaTek MT8113T (dual Cortex-A53, armhf)  |
| Architecture      | `arm-kobo-linux-gnueabihf` (32-bit)        |
| Kernel            | Linux ~4.19                                |
| Firmware          | Nickel 4.x                                 |
| Storage           | Internal eMMC, exposed as USB Mass Storage |
| Mount point       | `/mnt/onboard` (on-device), user-specified (on host) |
| Secure Boot       | Yes (no custom kernel modules)             |
| USB Mass Storage  | Available when connected via USB-C         |

## Security Model

The default configuration targets a child-safe, parent-managed device:

- **No Tailscale, no SSH, no listening services, no inbound connections.**
- **WiFi stays on** but is locked down. KOReader fetches book metadata (cover art, author info, descriptions) from Open Library and Google Books.
- **All Kobo/Rakuten telemetry is blocked** via `/etc/hosts` and a SQLite trigger.
- **The Kobo Store and built-in web browser are disabled** via Nickel's parental controls (set manually on device).
- **OTA firmware updates are disabled** to prevent hardening from being reset.
- **Books are loaded exclusively via USB** using Calibre's send-to-device or drag-and-drop.

## Architecture

```
koboctl
├── cmd/                    # Cobra command tree
│   ├── root.go
│   ├── provision.go        # Full provisioning workflow
│   ├── install.go          # Individual component installers
│   ├── status.go           # Detect and report device state
│   └── harden.go           # Security hardening command
├── internal/
│   ├── device/             # Device detection, model profiles
│   │   ├── detect.go       # USB mount detection, model identification
│   │   ├── profile.go      # Per-model device profiles
│   │   └── firmware.go     # Firmware version parsing
│   ├── installer/          # Component installers
│   │   ├── koreader.go
│   │   ├── nickelmenu.go
│   │   ├── kfmon.go
│   │   └── common.go       # Shared install logic (download, verify, extract)
│   ├── config/             # Configuration generators
│   │   └── nickelmenu.go   # NickelMenu entry DSL generation
│   ├── hardening/          # Security hardening operations
│   │   ├── hosts.go        # /etc/hosts telemetry blocklist boot script
│   │   ├── resolv.go       # DNS lockdown boot script (chattr +i)
│   │   ├── nickel.go       # Kobo eReader.conf hardening (direct write)
│   │   ├── parental.go     # Parental controls check + manual reminder
│   │   ├── sqlite.go       # KoboReader.sqlite analytics trigger
│   │   ├── devmode.go      # Telnet/devmode disable boot script
│   │   ├── mount.go        # KoboRoot.tgz directory guard
│   │   ├── plugins.go      # KOReader dangerous plugin removal
│   │   ├── runner.go       # Boot hook runner + KFMon on_boot config
│   │   └── verify.go       # Post-hardening verification checks
│   ├── manifest/           # Declarative config manifest
│   │   ├── parse.go
│   │   └── types.go
│   └── fetch/              # Artifact downloading and caching
│       ├── github.go       # GitHub Releases API client
│       ├── cache.go        # Local artifact cache (~/.cache/koboctl/)
│       └── verify.go       # SHA256 verification
├── profiles/               # Embedded device profiles
│   └── libra_colour.toml
├── go.mod
├── go.sum
└── cmd/koboctl/main.go
```

## Device Detection

When a Kobo is connected via USB and mounted, the tool identifies it by:

1. Scanning mounted filesystems for the presence of `/.kobo/` directory.
2. Reading `/.kobo/version` to extract model number, firmware version, serial number, and affiliate code.
3. Cross-referencing the model number against the embedded device profile database.
4. Detecting installed components by checking for known file paths.

The tool accepts an explicit mount point via `--mount` flag or auto-detects by scanning `/media/`, `/mnt/`, and `/run/media/$USER/` for Kobo volumes.

## Declarative Manifest

All device configuration is described in a TOML manifest file (`koboctl.toml`).

```toml
[device]
model = "libra-colour"
mount = ""

[koreader]
enabled = true
channel = "stable"
version = "latest"

[nickelmenu]
enabled = true
version = "latest"

  [[nickelmenu.entries]]
  location = "main"
  label = "KOReader"
  action = "dbg_toast"
  arg = "Starting KOReader..."
  chain = "cmd_spawn:quiet:/usr/bin/kfmon-ipc trigger koreader"

[kfmon]
enabled = true
version = "latest"

[hardening]
enabled = true

[hardening.network]
mode = "metadata-only"                  # "metadata-only" | "offline" | "open"
dns_servers = ["185.228.168.168", "185.228.169.168"]  # CleanBrowsing Family Filter
block_telemetry = true
block_ota = true
block_sync = true

[hardening.parental]
enabled = true
lock_store = true
lock_browser = true

[hardening.services]
disable_telnet = true
disable_ftp = true
disable_ssh = true

[hardening.filesystem]
noexec_onboard = false     # Cannot enable without breaking KOReader/Plato
disable_koboroot = true
remove_dangerous_plugins = true

[hardening.privacy]
block_analytics_db = true
hosts_blocklist = true
```

## CLI Commands

### `koboctl provision`

Full provisioning workflow.

```
koboctl provision [--manifest koboctl.toml] [--mount /media/KOBOeReader] [--dry-run]
```

**Execution order:**
1. Detect device (USB mount, model, firmware version)
2. Validate manifest
3. Fetch and cache all required artifacts (parallel)
4. Install KFMon
5. Install KOReader
6. Install NickelMenu
7. Apply security hardening (if `hardening.enabled = true`)
8. Print post-provision instructions (eject, reboot, set parental controls PIN)

### `koboctl harden`

Apply security hardening operations to an already-provisioned device.

```
koboctl harden [--manifest koboctl.toml] [--mount /media/KOBOeReader] [--dry-run]
```

Operations (in order):
1. Harden Nickel config (disable OTA, enable sideload mode, disable sync, disable debug services)
2. Install analytics SQLite trigger
3. Guard KoboRoot.tgz (replace with directory)
4. Remove dangerous KOReader plugins
5. Stage `/etc/hosts` blocklist boot script
6. Stage DNS lockdown boot script
7. Stage devmode/telnet disable boot script
8. Write boot hook runner (`run-hardening.sh`)
9. Write KFMon `on_boot` config

### `koboctl status`

Report current device state.

```
koboctl status [--mount /media/KOBOeReader] [--json]
```

Output:
```
Device:       Kobo Libra Colour (N428)
Firmware:     4.39.22801
Mount:        /media/KOBOeReader

Components:
  KOReader       installed
  KFMon          installed
  NickelMenu     installed

Hardening:
  Hosts blocklist            staged (pending boot)
  DNS lockdown               staged (pending boot)
  Devmode disable            staged (pending boot)
  Boot hook (KFMon)          ok
  Analytics trigger          ok
  KoboRoot guard             ok
  OTA updates                disabled
  Cloud sync                 disabled
  Sideload mode              enabled
  Dangerous plugins          0 found
  Parental controls          NOT SET
  ! Set manually: More -> Settings -> Accounts -> Parental Controls
```

### `koboctl install <component>`

Install or update a single component.

```
koboctl install koreader [--version v2024.11] [--channel nightly]
koboctl install nickelmenu
koboctl install kfmon
```

## Boot Hook Architecture

All hardening operations that modify the ext4 root filesystem (`/etc/hosts`,
`/etc/resolv.conf`, inetd config) must be staged as shell scripts on the FAT32
partition. USB Mass Storage only exposes FAT32.

### Script locations

```
/mnt/onboard/.adds/koboctl/
├── run-hardening.sh          # Master hook runner (called by KFMon on_boot)
├── harden-hosts.sh           # /etc/hosts blocklist
├── harden-dns.sh             # /etc/resolv.conf + chattr +i
├── harden-devmode.sh         # Kill telnetd, remove from inetd
└── hardening.log             # Runtime log (created on device at boot)
```

### KFMon on_boot trigger

`/mnt/onboard/.adds/kfmon/config/koboctl.ini` configures KFMon to run
`run-hardening.sh` once during boot initialisation (`on_boot = true`,
`on_boot_trigger = true`). A 1×1 transparent PNG at `/mnt/onboard/koboctl.png`
satisfies KFMon's trigger image requirement without adding a visible book.

### Persistence across firmware updates

Firmware updates overwrite the ext4 root filesystem, resetting `/etc/hosts`,
`/etc/resolv.conf`, and the KFMon binary. The FAT32 partition survives.
After a firmware update:
1. Re-run `koboctl provision` to reinstall KFMon.
2. KFMon `on_boot` re-executes the hardening scripts automatically.

## Artifact Management

### Sources

| Component   | Repository                          | Asset Pattern                             |
|-------------|-------------------------------------|-------------------------------------------|
| KOReader    | `koreader/koreader`                 | `koreader-kobo-arm-linux-gnueabihf-*.zip` |
| KFMon       | `NiLuJe/kfmon`                      | `KFMon-*.zip`                             |
| NickelMenu  | `pgaskin/NickelMenu`                | `KoboRoot.tgz`                            |

### Cache Layout

```
~/.cache/koboctl/
├── koreader/v2024.11/koreader-kobo-arm-linux-gnueabihf-v2024.11.zip
├── kfmon/v1.4.2/KFMon-v1.4.2.zip
└── nickelmenu/v0.6.0/KoboRoot.tgz
```

## Dependencies

| Dependency             | Purpose                                   |
|------------------------|-------------------------------------------|
| `cobra`                | CLI command framework                     |
| `go-github`            | GitHub Releases API for artifact fetching |
| `pelletier/go-toml`    | TOML manifest parsing                     |
| `modernc.org/sqlite`   | KoboReader.sqlite manipulation (pure Go, no CGO) |
| `golang.org/x/sync`    | errgroup for parallel artifact fetching   |

**CGO:** `CGO_ENABLED=0`. The `modernc.org/sqlite` driver is pure Go.

## Testing Strategy

### Unit Tests

- Manifest parsing and validation (including hardening config)
- Device profile matching and firmware version parsing
- NickelMenu config DSL generation
- Nickel INI file read-modify-write (preserves unmanaged keys)
- `/etc/hosts` blocklist: domain coverage, metadata domains not blocked
- SQLite analytics trigger: install, blocks inserts, idempotent
- KOReader plugin removal: only dangerous plugins removed
- KoboRoot guard: file replaced by directory, idempotent

### Integration Tests

Mount a loopback FAT32 filesystem with a mock Kobo structure and verify:
- All boot scripts written to `.adds/koboctl/`
- KFMon boot config written to `.adds/kfmon/config/koboctl.ini`
- Nickel config has hardened values
- Analytics trigger installed in SQLite
- KoboRoot.tgz is a directory
- No dangerous plugins remain
- Second run produces no changes (idempotency)

## References

- [koreader/koreader](https://github.com/koreader/koreader) — KOReader e-reader application
- [pgaskin/NickelMenu](https://github.com/pgaskin/NickelMenu) — Kobo menu injection
- [NiLuJe/kfmon](https://github.com/NiLuJe/kfmon) — Kobo filesystem launcher with on_boot support
- [baskerville/plato](https://github.com/baskerville/plato) — Minimalist Kobo reader
- [MobileRead Wiki — Kobo eReader hacks](https://wiki.mobileread.com/wiki/Kobo_eReader_hacks)
- [uwuu.ca — Custom Kobo Software](https://uwuu.ca/kobo/guide/custom-software/)
- CleanBrowsing Family Filter: `185.228.168.168` / `185.228.169.168`
