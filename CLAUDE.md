# CLAUDE.md — librito-kobo-sync

On-device Go agent that reads stock-Nickel highlights from a Kobo e-reader and
syncs them to the Librito web backend. **Work in progress** — Steps 1–3.5 (sync
core, pairing, udev WiFi-up autosync, resident watch daemon) are done and
hardware-verified; Steps 4–5 are not built yet (see _Roadmap_).

> **This is a public repo** (`librito-io/kobo-sync`, GPLv3). Keep personal
> data out of fixtures and commits (see _Fixtures_). Do not push to a remote
> without the user's explicit go-ahead — pause between commit and any remote
> action.

## The three Librito repos

The agent's code is self-contained, but its **correctness is defined by another
repo's API contract**. A session working only in this repo must know:

- **`librito-io/web`** — the SvelteKit web app + Supabase. Owns the import
  endpoint `POST /api/import/kobo` and its wire contract
  (`src/lib/server/import/kobo.ts`: payload shape, dedup key, validation,
  soft-delete semantics). **When the agent's behavior is in question, the truth
  is there, not here.** The catalog/cover resolver (which consumes the ISBN the
  agent sends) also lives in web.
- **`librito-io/reader`** — the PaperS3 ESP32 firmware (C++/PlatformIO). Shares
  **no code** with this agent. Holds a reference epub-OPF ISBN parser
  (`src/content/epub/EpubReader.cpp::parseIsbnFromOpf`) worth porting (see #503).
- **`librito-io/kobo-sync`** (this repo) — the Kobo companion. A _client_ of
  the web API, the same way the PaperS3 firmware and a browser are. The Kobo
  build plan + device facts live **here** in `docs/sync-build-plan.md`
  (local-only, gitignored — keeps device/dev specifics out of the public repo).

The Kobo is char-offset / chapter-path based, not word-index based. It is a
**separate write path** from the PaperS3's `/api/sync` (`processSync`); imported
rows leave the word-index columns NULL and render as plain quoted text.

## Architecture

```
main.go                 entrypoint + runXxx subcommand handlers; resolves prog name → dispatch()
route.go                route()      pure argv → decision: default | help | <subcommand> | unknown
command.go              commands     subcommand registry (single source) + renderHelp
dispatch.go             dispatch()   acts on the route decision; unknown command → error + exit 2
                        subcommands: sync (default) | pair | autosync | watch | status | about | sync-now
internal/sync/
  run.go      Run()         read → map → (post | dry-run); the orchestrator
  client.go   PostImport()  POST {url}/api/import/kobo, bearer token, full set
internal/kobo/
  read.go       ReadHighlights()          WAL-aware SQLite read → []RawBookmark
  signature.go  ReadHighlightSignature()  lightweight count + max(DateCreated)
internal/transform/
  transform.go  NormalizeISBN / NormalizeTimestamp / CleanText  (pure)
  item.go       RawBookmark, KoboImportItem, BuildItem()        (pure mapping)
internal/pair/    `pair` subcommand: on-device token acquisition (Step 2)
internal/autosync/  `autosync` subcommand: udev WiFi-up trigger → lock → connectivity-wait → sync.Run (Step 3)
  prober.go / lock.go / config.go / wait.go / log.go / syncer.go   (reused by watch)
internal/watch/   `watch` subcommand: resident inotify daemon, immediate sync while connected (Step 3.5)
  watch.go      Run()       single-instance lock → baseline → debounce → signature-diff → delegate to autosync.Run
  signature.go / decide.go / debounce.go   (pure core: grew / decide / debounceWait)
  watcher.go + inotify_linux.go / inotify_other.go   Watcher edge (x/sys/unix; linux impl + !linux stub)
```

**Pure core / impure edge split.** All mapping decisions live in `transform`
(pure, table-tested — every live-data edge case is a test row). The impure edges
are isolated and thin: `kobo` (SQLite), `sync` (HTTP), and the `autosync`/`watch`
trigger edges (sysfs/flock, inotify) — each behind an interface with a fake, so
the orchestration logic stays table-tested. This is deliberate: the tricky
correctness is in pure functions you can test exhaustively without a device or a
server.

## Load-bearing invariants (do NOT "fix" these without reading the why)

Each looks wrong to someone who doesn't know the context. They are correct.

1. **`content_id` = `VolumeID`, never `Bookmark.ContentID`.** `VolumeID` is the
   book root (`…/Book.epub`); `Bookmark.ContentID` is a per-chapter fragment
   (`…/Book.epub#…/ch01.xhtml`). The server synthesizes a book identity hash
   from `content_id` — using the fragment would split one book into many phantom
   books, one per chapter.
2. **ISBN is shape-validated; junk → nil.** Kobo's `content.ISBN` is whatever
   the epub's OPF primary identifier was — frequently `calibre:N`, an ASIN, or a
   `urn:uuid`, **even when the file contains a real ISBN elsewhere**. Passing
   junk through would trigger useless catalog lookups and risk two books
   colliding on the same fake id. `NormalizeISBN` accepts only ISBN-10/13 shape;
   everything else → nil → the book resolves by synthesized hash. (Recovering a
   real ISBN buried deeper in the OPF is tracked as web#503 — a cover-quality
   win, since ISBN is the primary cover-fetch signal.)
3. **`created_at` is normalized to UTC by appending `Z`.** Kobo writes
   `DateCreated` as naive-UTC with no zone designator (verified on a BST device:
   20:42 local stored as `19:42`). Sent as-is into a Postgres `timestamptz` it
   would be read as server-local and be wrong by the offset. Already-zoned values
   pass through untouched; unparseable values → nil (server defaults to `now()`).
4. **The read is WAL-aware.** Nickel keeps the DB in WAL mode and checkpoints on
   its own schedule, so new highlights live in the `-wal` sidecar until then.
   `ReadHighlights` opens read-only so it transparently sees un-checkpointed WAL
   rows. A reader that opened only the main `.sqlite` would silently miss recent
   highlights. (`internal/kobo/wal_test.go` guards this.)
5. **Soft-delete is server-owned; the agent is additive + idempotent.** Every
   run re-sends the **full** highlight set; the server dedups on
   `(book_id, source, source_uid)` and **omits `deleted_at` on conflict**, so a
   re-send never resurrects a web-deleted highlight. The agent does no local diff
   state. (Propagating a _device-side_ delete back to web is deferred — web#502.)
6. **v1 is highlights-only.** Kobo annotations (`Bookmark.Annotation`) are NOT
   synced. This is currently a scope choice, not a permanent rule — see
   _Open questions_.
7. **WiFi: silent path only — never the non-silent connect or a reboot.** The
   agent may bring WiFi up via `qndb -m wfmConnectWirelessSilently` and wait on
   the `wmNetworkConnected` signal. It must **never** call `wfmConnectWireless`
   (non-silent) or `pwrReboot`: on hardware (2026-06-06) the non-silent method
   popped a full-screen network-picker modal and **crashed Nickel into a reboot**;
   reboot-as-recovery stays forbidden for that crash itself (dropbear now
   boot-persists, so lost dev SSH is no longer the reason). Note two WiFi
   states: a _disabled_ radio can't be silently recovered, but the real-world
   state the agent faces is _enabled-but-disconnected_ (Nickel's battery-timer
   drop), which the silent path drives. Full probe results: `docs/sync-build-plan.md`
   ("Step-2 probe").

## Build & test

Requires Go 1.24+.

```sh
# static cross-compile for the Kobo (armv7l hard-float) — no C toolchain
# -X main.version stamps the version surfaced by `kobo-sync about` (single origin).
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -trimpath -ldflags="-s -w -X main.version=0.9.0" -o dist/librito-kobo-sync-armv7 .

go test ./...
```

Pure-Go: `modernc.org/sqlite` (no CGO) + Go's own TLS stack, so the binary is
statically linked and depends on nothing on the device — and TLS to librito.io
does not rely on the Kobo's (stale) CA store or busybox `ssl_client`.

### Fixtures — no personal reading data

Tests build **fabricated** `KoboReader.sqlite` fixtures in-test
(`internal/kobo/fixture_test.go`); there is no real DB blob in the repo. Keep it
that way — this repo may go public. Fixtures use fictional books/authors/text +
shape-valid fake ISBNs, but **preserve the real Kobo data shapes** the agent
must handle: the `epub#fragment` chapter ContentID, the literal `calibre:N`
junk-ISBN string, a genuinely ISBN-less sideload, leading-tab highlight text,
and an un-checkpointed WAL. The `calibre:N` literal is a format string, not data.

## Dev backbone (on-hardware loop)

The runnable dev harness is version-controlled: **`dev/README.md`** is the
committed runbook (SSH/dropbear boot stack, dev NickelMenu, `chown 0:0 /` fix,
`ForceWifiOn`) with the reference config files in `dev/`. **Device-specific
secrets** (IP/MAC/DHCP reservation) + the verified on-hardware findings stay in
`docs/sync-build-plan.md` (Step 0; local-only, gitignored). Essentials:

- **SSH:** custom dropbear. Dev **key auth (pubkey)** works and survives reboot
  (see _Boot-persistent SSH_ below). Persistent master:
  `ssh -M -S /tmp/kobo-ssh-master … root@<ip>`, then
  `ssh -S /tmp/kobo-ssh-master root@<ip> '<cmd>'`. (Full bring-up procedure:
  `dev/README.md`. Device IP/MAC: build plan — local-only.)
- **No `sftp-server`** → scp fails. Transfer via cat-pipe:
  `ssh -S "$CM" root@<ip> 'cat > /path' < localfile`.
- **WiFi drops by design** — Nickel powers the radio down on a battery timer,
  ignoring traffic. Dev workaround: set `ForceWifiOn=true` under
  `[DeveloperSettings]` in `Kobo eReader.conf` — **edit it over SSH** (the file is
  on `/mnt/onboard`, mounted rw; it is NOT USB-only), then **reboot to load**
  (Nickel reads the conf only at startup). The product agent (Step 3) must sync
  opportunistically on WiFi-up windows, never assume a standing link.
- **Boot-persistent SSH (verified 2026-06-08 by a real reboot test):** dropbear
  auto-starts on boot (rootfs udev `loop0` rule → `on-boot.sh`, started key-only)
  **and dev key auth survives reboot — no NickelMenu "SSH open" re-tap needed.**
  This took a one-time `chown 0:0 /` fix: `/` shipped owned by uid 501, which
  silently failed dropbear's pubkey home-ownership check, so before the fix every
  key was rejected and the tap WAS required (the tap blanks root's password +
  restarts `dropbear -B`). Device IP has been stable across reboots (DHCP lease,
  same MAC). Full mechanism + IP/MAC: build plan (local-only).
- **On-device path:** `/mnt/onboard/.adds/librito/kobo-sync`.
- **Local round-trip:** the device hits the Mac's dev server over LAN, so run
  `npm run dev -- --host` (not localhost-only) and point `--url` at the Mac's
  LAN IP. Mint a local device token by inserting a `devices` row against local
  Supabase (auth just hashes the token and looks it up).

Device under test: Kobo Libra Colour, Nickel 4.45.23697, kernel 4.9.77, armv7l,
MediaTek (Mark 13 / "Monza"). See the build plan for the full mod-stack state.

## Open questions / roadmap

Mark anything here clearly as **not built** when working — don't let aspiration
read as fact.

- **Annotations.** Out of v1 scope, but this is a deliberately-reopened design
  question, not a permanent rule. The PaperS3 couldn't author notes on-device (a
  hardware limit), which is _why_ "notes are web-created" became an invariant;
  the Kobo _can_ capture annotations, so it reopens the question: are Kobo
  annotations the same entity as web `notes`, or a separate thing? Undecided.
  (Overloading the existing `notes` table is blocked for technical reasons —
  RLS, the word-index down-path keying, the one-note-per-highlight unique. A
  separate table is the path if/when this ships.)
- **Steps 2–3.5 (built, hardware-verified):** on-device pairing (token to disk
  via NickelMenu) · udev WiFi-up auto-sync trigger · resident inotify watch daemon.
- **Steps 4–5 (not built):** FBInk status dashboard · Mac installer app.
- **Tracked issues:** web#502 (device→web delete propagation), web#503 (recover
  real ISBN from epub OPF when Nickel surfaces junk).

## Conventions

- Go standard style; `gofmt` clean, `go vet` clean before commit. CI
  (`.github/workflows/ci.yml`) gates build/test/vet/`gofmt`/golangci-lint on
  every PR.
- **TDD** for new behavior: failing test first, watch it fail, minimal pass.
  The pure transforms especially — every edge case is a test row.
- **Never push to a remote without explicit go-ahead.** Pause between commit
  and any remote action.

### PR & Commit Convention

Squash-merge default. The squash body concatenates the branch's commit messages
(`COMMIT_MESSAGES`), so **commit messages are the durable archeology**, not the
PR body.

- **Conventional Commits** for commit AND PR titles: `feat(scope):` `fix(scope):`
  `bug(scope):` `chore(scope):` `docs(scope):` `test(scope):` `perf(scope):`
  `refactor(scope):`. `bug` = user-facing defect fix, distinct from `fix`
  (internal regression). Scope is optional and free-form; a package name
  (`sync`, `transform`, `pair`, `kobo`) or `cli` are common.
- **Enforcement is CI-only** — this is a Go repo with no Node runtime, so there
  is no local husky hook (unlike `librito-io/web`).
  `.github/workflows/commitlint.yml` lints every commit in a PR;
  `.github/workflows/lint-pr-title.yml` gates the PR title. Both use the type
  list above; rules live in `commitlint.config.mjs`.
- **Subject ≤100 chars hard** (commitlint). Soft targets: ≤50 ideal, ≤72
  preferred. Body line length is not capped.
- **PR body** = ephemeral reviewer surface (summary + test plan). Do not
  duplicate commit-message archeology into it.
- Never use `--no-verify`.

### Issue tracking

All work is tracked in GitHub Issues on the shared org Project board
("Librito", `https://github.com/orgs/librito-io/projects/1`) spanning `web`,
`reader`, and `kobo-sync`. Issues opened from the templates auto-add to it.

- **File immediately** for incidental finds during a primary task — do not stash
  them in markdown trackers, and do not create follow-up `.md` docs.
- **Title:** imperative summary, no Conventional-Commits prefix. Type is set via
  the GitHub-native Issue Type field (`Bug` / `Feature` / `Chore` / `Docs`),
  area via an `area:*` label.
- **CLI flow** (older `gh` has no `--type` flag):
  ```bash
  ISSUE_URL=$(gh issue create --repo librito-io/kobo-sync --title "..." --label "area:sync" --body "...")
  gh api repos/librito-io/kobo-sync/issues/${ISSUE_URL##*/} -F type=Chore --silent
  ```
- **Body:** four `##` sections in order — `## Problem`, `## Solution` (mark
  `_unknown_` for bugs without a known fix), `## Discovery` (link the PR),
  `## Acceptance`.
- **Area labels:** `area:sync` `area:transform` `area:kobo` `area:pairing`
  `area:ui` `area:install` `area:build` `area:ci` `area:docs`.
- **Status labels:** `needs-triage` (auto-applied unless type + area both set,
  removed on triage), `blocked` (external dep + comment naming it), `deferred`
  (revival trigger documented in body, else close).
- **Cross-repo:** if work spans repos, file one issue per repo and cross-link
  the bodies; don't combine.
