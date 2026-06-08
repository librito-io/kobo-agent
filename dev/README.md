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

| File                    | Installs to                                | Purpose                                                                         |
| ----------------------- | ------------------------------------------ | ------------------------------------------------------------------------------- |
| `ssh/96-dropbear.rules` | `/etc/udev/rules.d/` (rootfs)              | boot trigger on `loop0` → launcher                                              |
| `ssh/boot.sh`           | `/usr/local/dropbear/` (rootfs)            | udev launcher; `setsid`-detaches so udev can't reap dropbear                    |
| `ssh/on-boot.sh`        | `/usr/local/dropbear/` (rootfs)            | starts dropbear **key-only** (`-s -g`); idempotent                              |
| `ssh/ssh-key.sh`        | run once                                   | plants your pubkey (`$LIBRITO_DEV_PUBKEY`) into root's `~/.ssh`                 |
| `ssh/ssh-fix.sh`        | run once                                   | repairs `on-boot.sh` (strips bad `-E`), runs `chown 0:0 /`, fixes `/.ssh` perms |
| `ssh/ssh-open.sh`       | `/mnt/onboard/.adds/librito/` (user part.) | NickelMenu blank-password recovery hatch                                        |
| `nm-config`             | `/mnt/onboard/.adds/nm/config`             | dev NickelMenu (SSH open/diag + smoke tests)                                    |

**Third-party mods you install (public, not in this repo):**

- **NickelMenu** v0.6.0 — `https://github.com/pgaskin/NickelMenu/releases/download/v0.6.0/KoboRoot.tgz`
- **NickelDBus** 0.2.0 — `https://github.com/shermp/NickelDBus/releases/download/0.2.0/KoboRoot.tgz`

Both assets are literally named `KoboRoot.tgz` — **rename on disk before copying**
so they don't overwrite each other (e.g. `KoboRoot-nm.tgz` / `KoboRoot-ndb.tgz`;
the device unpacks any `*.tgz` you drop in `.kobo/`). Pin these versions — newer
ones are untested against this firmware.

**The dropbear `KoboRoot.tgz` is NOT in version control and cannot be built from
this runbook** — the scripts above are inert without it. Reproducing it is the
one true gap (tracked: kobo-agent#26). Rough recipe: cross-compile dropbear
(v2025.89, modern, with ML-KEM/sntrup761) from `obynio/kobopatch-ssh`'s toolchain
(the `obynio/kobo-toolchain:crosstools` image is **arm64-native** — do NOT force
`--platform linux/amd64` on Apple Silicon), then package a `KoboRoot.tgz` that
installs the binaries to `/usr/local/bin/` **and bundles the three rootfs files
from `ssh/` here** (`96-dropbear.rules` → `/etc/udev/rules.d/`, `boot.sh` +
`on-boot.sh` → `/usr/local/dropbear/`). Until #26 lands, you need a prebuilt copy
of that tgz from a known-good machine.

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
open" tap. One-time, non-recursive, idempotent; persists like the rootfs rules
(reversible with `chown 501:root /`). **`ssh-fix.sh` runs this for you** — it is
documented here because the _why_ is load-bearing institutional knowledge, not
because you must run it by hand:

```sh
chown 0:0 /
```

## Prerequisites (do these before bring-up)

A factory-fresh device cannot be reached over SSH until it is on your LAN, and a
firmware drift silently invalidates this whole recipe. So, first:

1. **Complete Nickel onboarding and join your WiFi.** A factory-reset Libra Colour
   boots into the activation wizard — go through it and connect to the same network
   as your dev machine. SSH-over-LAN below assumes the device is reachable.
   ⚠️ Activation may require signing into (or creating) a Kobo account before it
   will finish and mount the USB partition; have one ready. The device must reach
   "Home" at least once so it processes the `.kobo/*.tgz` you drop in the next step.
2. **🛑 Do NOT let it firmware-update.** This recipe is verified on Nickel
   4.45.23697 only (see the banner). If the wizard offers an OTA, decline it; after
   setup, confirm auto-update is OFF (Settings → Device information → Automatic
   updates). A surprise OTA replaces the rootfs and breaks the mod stack — re-verify
   everything if the firmware ever moves.
3. **Have an SSH keypair on your dev machine.** If `~/.ssh/id_ed25519.pub` does not
   exist, create one: `ssh-keygen -t ed25519`. (The bring-up plants this exact file;
   adjust the path below if yours differs.)
4. **Run the SSH steps from this `dev/` directory** — the `cat`-pipe commands read
   relative paths like `ssh/ssh-key.sh`. `cd` into `dev/` first.

## Fresh-device bring-up (first touch — USB + taps, then SSH)

Chicken-and-egg: you can't SSH in to set up SSH. The first touch is manual.

1. **Install the mod stack via USB.** Plug the Kobo into your Mac; it mounts as
   `/Volumes/KOBOeReader` (confirm with `ls /Volumes/` — relabel the path below if
   yours differs). Drop **three** renamed tgz files into
   `/Volumes/KOBOeReader/.kobo/`: the prebuilt dropbear tgz, the NickelMenu tgz, and
   the NickelDBus tgz (URLs + the rename-so-they-don't-collide note are above). Eject;
   the device unpacks each on boot and deletes the tgz. (Dropbear + NickelMenu are
   required for SSH; **NickelDBus is not load-bearing for the SSH harness** — only
   the dev menu's "NDB Test toast" smoke item and later on-device UI use it. Skip it
   if you only want a shell.)
2. **Place the dev files via USB** onto the user partition:
   - `nm-config` → `/Volumes/KOBOeReader/.adds/nm/config`
   - `ssh/ssh-open.sh` → `/Volumes/KOBOeReader/.adds/librito/ssh-open.sh`

   ⚠️ **macOS pollutes the device.** Finder writes `._`-prefixed AppleDouble
   sidecars (e.g. `._config`) next to every file. NickelMenu loads **every** file in
   `.adds/nm/`, so a stray `._config` duplicates the whole menu. After copying, run
   `dot_clean /Volumes/KOBOeReader` (or `find /Volumes/KOBOeReader -name '._*' -delete`)
   before ejecting. Eject.

3. **Reboot** (Nickel: power button → _Power off_ / _Restart_, or once SSH is up,
   `reboot` over SSH — **never** `pwrReboot`, it crashes Nickel; invariant #7). The
   `loop0` udev rule starts key-only dropbear — but your key isn't planted and `/`
   ownership isn't fixed yet, so key auth won't work _yet_.
4. **Tap NickelMenu → Librito → "SSH open."** Blanks root pw, starts `dropbear -B`;
   the dialog prints the device's IP. You can now SSH in with `BatchMode` (blank/none
   auth). **If this menu item is absent**, the NickelMenu tgz didn't unpack (re-check
   step 1) — there is no other entry path to a fresh device, so don't reboot past
   this until the menu appears.
5. **SSH in and make key auth permanent** (run from `dev/`):

   ```sh
   # IP: from the "SSH open" dialog, or your router's DHCP table, or match the
   #   device's wlan0 MAC against the ARP table: arp -a | grep -i <your-mac-prefix>
   CM=/tmp/kobo-ssh-master; IP=<device-ip>
   # first connect from a new machine has no known_hosts entry → accept-new (or
   # the later BatchMode call hard-fails "Host key verification failed")
   ssh -M -S "$CM" -o ServerAliveInterval=10 -o StrictHostKeyChecking=accept-new -fN root@$IP
   S(){ ssh -S "$CM" root@$IP "$@"; }

   # push the scripts (no sftp-server on device → cat-pipe everything)
   S 'cat > /mnt/onboard/.adds/librito/ssh-key.sh' < ssh/ssh-key.sh
   S 'cat > /mnt/onboard/.adds/librito/ssh-fix.sh' < ssh/ssh-fix.sh

   # repair on-boot.sh + chown 0:0 / (verified, not blind) + fix /.ssh perms
   S 'sh /mnt/onboard/.adds/librito/ssh-fix.sh'

   # plant your pubkey for key-only auth
   S "LIBRITO_DEV_PUBKEY='$(cat ~/.ssh/id_ed25519.pub)' sh /mnt/onboard/.adds/librito/ssh-key.sh"
   ```

6. **Reboot and confirm key-only auth with no tap** (the host key is already in
   `known_hosts` from step 5, so BatchMode won't trip on it):
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
reads the conf **only at startup**, so reboot to load.

On a fresh device the `[DeveloperSettings]` section and the `ForceWifiOn` line
often **don't exist yet** (Nickel writes the conf lazily), so a plain `sed`
substitution silently no-ops. Add-or-replace instead:

```sh
CONF='/mnt/onboard/.kobo/Kobo/Kobo eReader.conf'
S "
  if grep -q '^ForceWifiOn=' '$CONF'; then
    sed -i 's/^ForceWifiOn=.*/ForceWifiOn=true/' '$CONF'
  elif grep -q '^\[DeveloperSettings\]' '$CONF'; then
    sed -i 's/^\[DeveloperSettings\]/[DeveloperSettings]\nForceWifiOn=true/' '$CONF'
  else
    printf '\n[DeveloperSettings]\nForceWifiOn=true\n' >> '$CONF'
  fi
  grep -A1 'DeveloperSettings' '$CONF'
"
# then reboot to load
```

⚠️ Dev-only — it costs battery. The **product** agent must never assume a standing
link; it syncs opportunistically on WiFi-up windows (build plan Step 3).

## Daily loop cheatsheet

Build the agent first (`CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build …` —
see the repo root CLAUDE.md "Build & test"). Then, from the repo root:

```sh
CM=/tmp/kobo-ssh-master; IP=<device-ip>   # the IP from bring-up (lease is stable)
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
