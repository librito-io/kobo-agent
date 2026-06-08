# Dev harness — on-device SSH + NickelMenu loop

The on-hardware development loop for `librito-kobo-agent`: root SSH over custom
dropbear, a dev NickelMenu for recovery/smoke-tests, and the WiFi-stays-up tweak.
**None of this ships** — it is contributor tooling for working on a real Kobo. The
user-facing product (the agent, `../nm/librito`, `../udev/*`) is separate.

> **This is a recipe, not an auto-installer.** The files in this directory are
> **reference snapshots last verified on Nickel 4.45.23697 / kernel 4.9.77,
> Kobo Libra Colour (MediaTek "Monza", device id 390).** Device access on a Kobo
> is firmware-version-fragile — **re-verify after any firmware bump.** There is
> deliberately no one-shot bootstrap script: the first-touch path needs USB +
> physical taps (see the chicken-and-egg note), and a script that claimed to
> reproduce this across firmwares would lie.

Device-specific secrets (the device IP/MAC, DHCP reservation) live in the
local-only build plan (`docs/agent-build-plan.md`, gitignored), **not here**.

## What's in here

| File                    | Installs to                                | Purpose                                                         |
| ----------------------- | ------------------------------------------ | --------------------------------------------------------------- |
| `ssh/96-dropbear.rules` | `/etc/udev/rules.d/` (rootfs)              | boot trigger on `loop0` → launcher                              |
| `ssh/boot.sh`           | `/usr/local/dropbear/` (rootfs)            | udev launcher; `setsid`-detaches so udev can't reap dropbear    |
| `ssh/on-boot.sh`        | `/usr/local/dropbear/` (rootfs)            | starts dropbear **key-only** (`-s -g`); idempotent              |
| `ssh/ssh-key.sh`        | run once                                   | plants your pubkey (`$LIBRITO_DEV_PUBKEY`) into root's `~/.ssh` |
| `ssh/ssh-fix.sh`        | run once                                   | repairs `on-boot.sh` (strips bad `-E`) + `/.ssh` perms          |
| `ssh/ssh-open.sh`       | `/mnt/onboard/.adds/librito/` (user part.) | NickelMenu blank-password recovery hatch                        |
| `nm-config`             | `/mnt/onboard/.adds/nm/config`             | dev NickelMenu (SSH open/diag + smoke tests)                    |

**Not in version control (yet):** the cross-compiled `dropbear` + `dropbearkey`
binaries and the self-installing `KoboRoot.tgz`. On a fresh device the scripts
above are inert without that binary. Build recipe: cross-compile dropbear
(v2025.89, modern, with ML-KEM/sntrup761) from `obynio/kobopatch-ssh`'s toolchain
— the `obynio/kobo-toolchain:crosstools` image is **arm64-native** (do NOT force
`--platform linux/amd64` on Apple Silicon). Packaging that recipe into VC is
tracked separately (it overlaps the Step-5 installer's bundled `KoboRoot.tgz`).

## Two SSH auth paths (know the difference)

1. **Key-only, boot-persistent (the steady state).** `on-boot.sh` starts
   `dropbear -s -g` (password auth OFF). Your pubkey in `/.ssh/authorized_keys`
   authenticates you, with **no tap**, ~13 s after a cold boot. This is what you
   want day-to-day. Survives reboot (rootfs udev rule). **Requires the
   `chown 0:0 /` fix below**, or every key is rejected.
2. **Blank-password recovery hatch (fallback only).** The NickelMenu "SSH open"
   tap runs `ssh-open.sh`, which blanks root's password and restarts
   `dropbear -B`. Use it only when key auth is broken (fresh device, or before the
   chown fix). It does **not** survive a reboot. 🛑 Must be stripped before any
   production/installer ship.

### The load-bearing fix: `chown 0:0 /`

Root's home directory is `/` on this device. dropbear's strict pubkey check
requires the home dir to be owned by root — and `/` **shipped owned by uid 501**,
so the key-only path silently rejected every key (log: `/ must be owned by user
or root... Exit before auth`). That is why earlier sessions kept needing the "SSH
open" tap. One-time fix, persists like the rootfs rules (reversible with
`chown 501:root /`):

```sh
chown 0:0 /
```

## Fresh-device bring-up (first touch — USB + taps, then SSH)

Chicken-and-egg: you can't SSH in to set up SSH. The first touch is manual.

1. **Install the mod stack via USB.** Drop the dropbear `KoboRoot.tgz` (see build
   recipe above) plus NickelMenu + NickelDBus `KoboRoot.tgz` files into
   `/Volumes/KOBOeReader/.kobo/`, eject. Device unpacks on boot. (NickelMenu /
   NickelDBus URLs + install notes: build plan.)
2. **Place the dev files via USB** onto the user partition:
   - `nm-config` → `/Volumes/KOBOeReader/.adds/nm/config`
   - `ssh/ssh-open.sh` → `/Volumes/KOBOeReader/.adds/librito/ssh-open.sh`
     Eject.
3. **Reboot.** The `loop0` udev rule starts key-only dropbear — but your key isn't
   planted and `/` ownership isn't fixed yet, so key auth won't work _yet_.
4. **Tap NickelMenu → Librito → "SSH open."** Blanks root pw, starts
   `dropbear -B`. You can now SSH in with `BatchMode` (blank/none auth).
5. **SSH in and make key auth permanent:**

   ```sh
   CM=/tmp/kobo-ssh-master; IP=<device-ip from build plan>
   ssh -M -S "$CM" -o ServerAliveInterval=10 -fN root@$IP
   S(){ ssh -S "$CM" root@$IP "$@"; }

   # plant your key (no sftp-server on device → cat-pipe everything)
   S 'cat > /mnt/onboard/.adds/librito/ssh-key.sh' < ssh/ssh-key.sh
   S 'cat > /mnt/onboard/.adds/librito/ssh-fix.sh' < ssh/ssh-fix.sh
   S "LIBRITO_DEV_PUBKEY='$(cat ~/.ssh/id_ed25519.pub)' sh /mnt/onboard/.adds/librito/ssh-fix.sh"
   S "LIBRITO_DEV_PUBKEY='$(cat ~/.ssh/id_ed25519.pub)' sh /mnt/onboard/.adds/librito/ssh-key.sh"

   # the load-bearing fix
   S 'chown 0:0 /'
   ```

6. **Reboot and confirm key-only auth with no tap:**
   ```sh
   ssh -o BatchMode=yes root@$IP 'id; ls -ld /'   # expect uid=0, / owned by root
   ```

After this, every cold boot answers on port 22 with key auth, no tap.

## Keep WiFi up for the dev loop (`ForceWifiOn`)

Nickel powers the WiFi radio down on its own battery/idle timer regardless of
traffic (a gateway-ping keepalive does **not** stop it). For an SSH session that
stays up, set `ForceWifiOn=true` under `[DeveloperSettings]` in
`/mnt/onboard/.kobo/Kobo/Kobo eReader.conf` (note the nested `Kobo/`). The file is
on `/mnt/onboard` (mounted **rw** — edit over SSH, it is NOT USB-only); Nickel
reads the conf **only at startup**, so reboot to load:

```sh
S "sed -i 's/^ForceWifiOn=false/ForceWifiOn=true/' '/mnt/onboard/.kobo/Kobo/Kobo eReader.conf'"
# then reboot
```

⚠️ Dev-only — it costs battery. The **product** agent must never assume a standing
link; it syncs opportunistically on WiFi-up windows (build plan Step 3).

## Daily loop cheatsheet

```sh
CM=/tmp/kobo-ssh-master; IP=<device-ip from build plan>
ssh -M -S "$CM" -o ServerAliveInterval=10 -fN root@$IP   # one auth, reused
S(){ ssh -S "$CM" root@$IP "$@"; }

# no sftp-server → push files via cat-pipe
S 'cat > /mnt/onboard/.adds/librito/agent' < dist/librito-kobo-agent-armv7
S 'chmod +x /mnt/onboard/.adds/librito/agent'

S '/mnt/onboard/.adds/librito/agent sync --dry-run'      # run it
```

## Persistence scope

Rootfs changes (udev rules, `chown /`) survive **reboot** but **not a firmware
update** (rootfs is replaced) — the Step-5 installer reinstalls them. Everything
under `/mnt/onboard/.adds/librito/` (user partition) survives a firmware update.
