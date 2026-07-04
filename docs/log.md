# koboctl — Decision Log

## 2026-07-04 — Add interactive Bubble Tea TUI

**Decision:** Added an interactive TUI (`internal/tui/`, `cmd/tui.go`). Bare
`koboctl` on a TTY launches it; `koboctl tui` too; all existing subcommands stay
headless for scripting (bare invocation when piped prints help). The TUI shows a
live device dashboard (2s auto-detect poll), a full collapsible manifest editor
(every option), runs provision/install/harden/backup with output streamed into a
log pane, previews provision/harden as a dry-run before applying, and saves with
a config diff. Convenience features: live auto-detect, dry-run preview, diff on
save, plugin browser.

**Rationale:** The headless CLI made it unclear what state the device was in or
what the tool was doing. A dashboard + editor + live action log turns koboctl
into a dynamic configuration manager. Bubble Tea was the requested stack.

**Key implementation choices:**
- **No import cycle:** `cmd` imports `internal/tui` for the command, so the TUI
  must not import `cmd`. Provision/harden orchestration is injected via a
  `tui.Actions{Provision, Harden}` callback struct; installers/backup/device are
  called directly (no cycle). Extracted `cmd.RunProvision` from the provision
  RunE so both the cobra command and the TUI drive the same code.
- **Live log without refactoring installers:** installers/hardening print to
  `os.Stdout`/`os.Stderr`. `runAction` redirects both to an `os.Pipe` during an
  action, scans lines, and `prog.Send`s them as `logLineMsg`. Bubble Tea's
  renderer captured its own output handle at program creation, so it keeps
  drawing to the terminal; a single-action busy guard makes the global swap safe.
  Chosen over changing every installer signature to take an `io.Writer`.
- **Faithful save:** templated `mount` in `internal/init/template.go` (was
  hardcoded `""`) so the editor round-trips `Device.Mount`. `noexec_onboard`
  stays pinned `false` and is shown read-only (unsupported by design).
- Config tree binds getter/setter closures over the live `*manifest.Manifest`;
  save validates via `manifest.ValidateManifest`, renders via `initcmd.Render`,
  and shows a line-level LCS diff before writing.

**Alternatives considered:**
- Separate `tui` subcommand only (no bare-root launch): rejected — bare `koboctl`
  opening the dashboard is the discoverable default; scripting is preserved via
  the isatty guard.
- Refactor installers to stream via an injected writer: rejected for v1 — larger
  blast radius; the pipe-capture is localized to the TUI.

<!-- TODO:FEATURE — TUI: NickelMenu entry editor (entries are hand-edited in TOML
     for now); restore file picker; scrollable modals for long diffs. -->

## 2026-07-04 — Add KOReader plugin support (dynamic_panelzoom)

**Decision:** Added a declarative KOReader plugin installer. `[koreader].plugins`
takes a list of names (`"name"` or `"name@vX.Y.Z"`) resolved against a built-in
registry (`internal/plugins`). Plugins are fetched from GitHub releases and
extracted into `.adds/koreader/plugins/<name>.koplugin/` after KOReader is
installed. Seeded the registry with `dynamic_panelzoom`
(JorgeTheFox/koreader-dynamic-panelzoom) and enabled it in `koboctl.toml`.

**Rationale:** The target reader reads comics/manga; dynamic_panelzoom adds
panel-by-panel navigation. The plugin publishes proper GitHub releases with a
stable `dynamic_panelzoom.koplugin.zip` asset, so it reuses the existing fetch
path (KOReader/NickelMenu) unchanged. The registry lives in its own
dependency-free package so both the manifest validator and installer can use it
without an import cycle (installer already imports manifest). dynamic_panelzoom
is not in `DangerousPlugins()`, so hardening's plugin removal leaves it intact.

**Alternatives considered:**
- Embed the zip in the binary (like KFMon): rejected — KFMon is embedded only
  because it has no GitHub releases; embedding here means re-vendoring on every
  update for no offline benefit (provision already needs network).
- Generic `[[koreader.plugins]]` table with repo/asset/version fields: rejected
  for now — pushes asset-glob/remap correctness onto hand-edited TOML; the named
  registry keeps those details in tested Go. Revisit if self-service is wanted.

<!-- TODO:FEATURE — plugin uninstall/GC: prune .adds/koreader/plugins entries no
     longer listed in the manifest. -->

## 2026-03-31 — Add hardening module, remove Tailscale/SSH/OPDS/Calibre

**Decision:** Added `internal/hardening/` package implementing security hardening for a child-safe Kobo device. Removed Tailscale, SSH, OPDS catalog, and Calibre wireless sync from scope and manifest types.

**Rationale:** The target device is configured for a 12-year-old. The security model requires no inbound connections, blocked telemetry, family-safe DNS, parental controls, and OTA disabled. Tailscale/SSH increase attack surface and are not needed when all management is via USB. OPDS/Calibre wireless are not needed when books are loaded exclusively via USB drag-and-drop or Calibre send-to-device.

**Alternatives considered:**
- Keep Tailscale as optional feature: rejected — adds CGO dependency (wireguard-go), increases attack surface, and the use case does not require it.
- Use a firewall (iptables) instead of /etc/hosts: rejected — requires kernel module support (not guaranteed on stock Kobo), more fragile than hosts-based blocking.
- noexec on /mnt/onboard: rejected — all hacked Kobo software (KFMon, KOReader, NickelMenu, Plato) executes from FAT32 `.adds/`, making this impractical for v1.

<!-- TODO — noexec_onboard: revisit if a bind-mount or overlay approach can separate
     the .adds/ execution area from user-writable space without breaking KOReader. -->

<!-- TODO:FEATURE — koboctl update-firmware: command to remove KoboRoot.tgz guard,
     apply a legitimate firmware update, then re-apply hardening. -->

<!-- TODO:FEATURE — koboctl cache: subcommand to pre-download all artifacts for
     offline provisioning (list, fetch, clean, path). -->
