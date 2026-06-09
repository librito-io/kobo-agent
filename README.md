# librito-kobo-sync

On-device agent that reads stock-Nickel highlights from a Kobo e-reader and
syncs them to the Librito web backend (`POST /api/import/kobo`).

This is the **mod-independent sync core** (Step 1 of the Kobo companion). It
takes a device token and a path to `KoboReader.sqlite`, reads the visible
highlights, and POSTs them to Librito. No UI, no pairing, no auto-trigger —
those are later steps (see _Roadmap_).

## What it does

1. Reads highlights from `KoboReader.sqlite` (the Kobo's `Bookmark` table,
   joined to `content` for book + chapter metadata). WAL-aware: sees highlights
   Nickel has written to the WAL but not yet checkpointed.
2. Maps each to the Librito import wire shape:
   - `source_uid` ← Kobo `BookmarkID` (stable dedup key)
   - `content_id` ← `VolumeID` (book root, so a book stays one book)
   - ISBN shape-validated — junk (`calibre:N`, ASIN, `urn:uuid`) → null so the
     book resolves by a synthesized hash instead of a bogus catalog lookup
   - `created_at` ← `DateCreated`, normalized to UTC (Kobo writes naive-UTC)
   - `chapter_title` ← the chapter `content` row (rendered in the web UI)
3. POSTs the full set to `{url}/api/import/kobo` with a device bearer token.
   The import is idempotent server-side (dedup on
   `(book_id, source, source_uid)`), so the agent always re-sends everything —
   no local diff state. A highlight deleted on the web is never resurrected.

v1 syncs **highlight text only** — Kobo annotations are out of scope for now.

## Build

Requires Go (1.24+).

```sh
# host build (for local testing)
go build -o librito-kobo-sync .

# static cross-compile for the Kobo (armv7l, hard-float)
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -ldflags="-s -w" -o dist/librito-kobo-sync-armv7 .
```

The binary is statically linked (pure-Go SQLite via `modernc.org/sqlite`, Go's
own TLS stack) — no C toolchain, no runtime dependencies on the device.

## Usage

```sh
librito-kobo-sync \
  --db   /mnt/onboard/.kobo/KoboReader.sqlite \
  --url  https://librito.io \
  --token sk_device_xxxxx

# inspect what would be sent, without a token or a POST:
librito-kobo-sync --db ./KoboReader.sqlite --dry-run
```

The token may also be supplied via `LIBRITO_TOKEN`. Defaults: `--db` is the
standard Kobo path, `--url` is `https://librito.io`.

## On-device install (dev)

No `sftp-server` on stock Kobo; copy over an SSH master with a cat-pipe:

```sh
ssh -S "$CM" root@<kobo-ip> \
  'mkdir -p /mnt/onboard/.adds/librito && cat > /mnt/onboard/.adds/librito/kobo-sync && chmod +x /mnt/onboard/.adds/librito/kobo-sync' \
  < dist/librito-kobo-sync-armv7
```

## Tests

```sh
go test ./...
```

Tests run against fabricated `KoboReader.sqlite` fixtures built in-test
(`internal/kobo/fixture_test.go`) — no real personal reading data in the repo.
The fixtures preserve the real Kobo data _shapes_ the agent must handle: the
`epub#fragment` chapter ContentID, the `calibre:N` junk-ISBN string, a
genuinely ISBN-less sideload, leading-tab highlight text, and an
un-checkpointed WAL.

## Roadmap

| Step | Status | What                                           |
| ---- | ------ | ---------------------------------------------- |
| 1    | ✅     | Sync core (read SQLite → POST) — this repo     |
| 2    | —      | On-device pairing (NickelMenu → token to disk) |
| 3    | —      | udev WiFi-up auto-sync trigger                 |
| 4    | —      | FBInk status dashboard                         |
| 5    | —      | Mac installer app                              |

Build plan + device facts: [`docs/sync-build-plan.md`](docs/sync-build-plan.md).
Install prerequisites (NickelDBus, NickelMenu): [`docs/install.md`](docs/install.md).
Import endpoint + wire contract: `librito-io/web` `src/lib/server/import/kobo.ts`.

## License

Copyright 2026 Nathan Fushia.

Licensed under the [GNU General Public License v3.0](LICENSE). This program is
free software: you can redistribute it and/or modify it under the terms of the
GPLv3. It is distributed WITHOUT ANY WARRANTY. Contributions welcome — the agent
is a device adapter, and more device adapters are the point.
