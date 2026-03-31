# koboctl — Decision Log

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
