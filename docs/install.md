# Install prerequisites

The agent is a pure-Go binary, but on-device it relies on two third-party Kobo
mods for its UI hooks. They are **install-time prerequisites**, not bundled with
this repo — they are separately-licensed (GPL) projects, so install them from
their upstream releases rather than expecting a copy here.

| Mod            | Role for the agent                                                    | Upstream                                                                                                              | Tested version          |
| -------------- | --------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| **NickelMenu** | Menu entry that launches the agent / pairing (`cmd_spawn`)            | <https://pgaskin.net/NickelMenu> · [releases](https://github.com/pgaskin/NickelMenu/releases)                         | **v0.6.0** (2025-12-07) |
| **NickelDBus** | Native dialog + toast for the pairing code and sync feedback (`qndb`) | [shermp/NickelDBus](https://github.com/shermp/NickelDBus) · [releases](https://github.com/shermp/NickelDBus/releases) | **0.2.0** (2021-11)     |

Both were verified GO on a **Kobo Libra Colour, firmware 4.45.23697, kernel
4.9.77** (2026-06-04). NickelDBus 0.2.0 is stale/unmaintained but its install
hook fires and `qndb` dialog/toast render correctly on this build; auto-update is
kept OFF so a forced firmware bump can't break the toast symbol.

## Installing a mod

Each ships as a self-installing `KoboRoot.tgz`. Copy it to `/mnt/onboard/.kobo/`
on the device (over USB or SSH) and reboot — Nickel consumes it on boot.

```sh
# over SSH (no sftp on stock Kobo — cat-pipe the file)
ssh root@<kobo-ip> 'cat > /mnt/onboard/.kobo/KoboRoot.tgz' < NickelMenu-KoboRoot.tgz
ssh root@<kobo-ip> 'reboot'
```

## Agent binary

Cross-compile for the Kobo (32-bit ARMv7, static):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -ldflags="-s -w" -o dist/librito-kobo-sync-armv7 .
```

On-device path: `/mnt/onboard/.adds/librito/kobo-sync`.
