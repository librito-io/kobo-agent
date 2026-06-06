# Kobo highlight-sync agent — build plan

On-device agent that reads stock-Nickel highlights from a Kobo and POSTs them to
the Librito web backend (`POST /api/import/kobo`, already shipped). Each step
below is sized to run as **one fresh session / one task**. Steps are ordered by
dependency; verified facts and open unknowns are called out per step so a fresh
session starts from confirmed ground.

**Status legend:** ✅ verified · ⚠️ caveat · ❓ unverified (on-hardware confirm needed)

---

## Device under test (confirmed identity)

- **Model:** Kobo Libra Colour (2024, Kaleido 3 colour e-ink, **MediaTek SoC** —
  Mark 13 / codename "Monza", device id 390; NOT sunxi, corrected on-HW 2026-06-04)
- **Software (Nickel):** 4.45.23697 (`f576aa4ee9`, build 2026-05-25)
- **Kernel:** 4.9.77 #1 SMP PREEMPT
- Firmware **auto-update is OFF** (deliberate — protects the mod stack from a
  surprise OTA to an unsupported firmware).

---

## Mod-stack feasibility — TESTED ON HARDWARE 2026-06-04 (GO/GO)

| Mod                                              | Build tested           | Result                                                                                                                                       | What it unblocks                                                        |
| ------------------------------------------------ | ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| **NickelMenu** v0.6.0 (2025-12-07)               | KoboRoot.tgz, 66939 B  | ✅ PASS — `cmd_output :uname -a` rendered, no dlsym error                                                                                    | `cmd_spawn` runs arbitrary scripts → manual sync trigger + WiFi actions |
| **NickelDBus** 0.2.0 (2021-11, unmaintained tag) | KoboRoot.tgz, 121353 B | ✅ PASS — install hook fired (`.adds/nickeldbus` created); toast `qndb -m mwcToast` AND dialog `qndb -m dlgConfirm*` BOTH rendered (Step 0a) | On-screen feedback layer — toast + confirm-dialog both confirmed        |

Notes:

- NickelDBus binds Nickel functions by **mangled C++ symbol name via dlsym**
  (e.g. `_ZN20MainWindowController5toastERK7QStringS2_i`). Name-based resolution
  survives firmware bumps as long as the function signature is unchanged — which
  is why the stale 2021 build still works on 2026 firmware. Issue #19's "won't
  install on Libra Colour" install-trigger bug does **not** affect this build.
- ⚠️ NickelDBus 0.2.0 is unmaintained. A future forced firmware update could
  break the toast/dialog symbol. Mitigations: auto-update is OFF; toast is
  best-effort (agent core does not depend on it); fallback = rebuild from source
  (NickelHook is current, commits 2025-12-13). This is **garnish, never the
  backbone.**

### Download URLs (verified resolve, GitHub API)

- NickelMenu: `https://github.com/pgaskin/NickelMenu/releases/download/v0.6.0/KoboRoot.tgz`
- NickelDBus: `https://github.com/shermp/NickelDBus/releases/download/0.2.0/KoboRoot.tgz`

Both assets are literally named `KoboRoot.tgz` — rename on disk before copying so
they don't overwrite each other. Install = drop into `/Volumes/KOBOeReader/.kobo/`,
eject, device unpacks on boot and deletes the tgz.

---

## Pairing — REUSE the existing PaperS3 flow unchanged ✅

The PaperS3 pairing flow in `librito-io/web` is **device-agnostic** (OAuth 2.0
Device Authorization Grant). A Kobo is just another device to the backend — **no
backend changes needed.** Three HTTP calls:

1. **Device → `POST /api/pair/request`** body `{ hardwareId }` (a UUID the agent
   generates+persists once). Unauthenticated. Returns
   `{ code, pairingId, pollSecret, expiresIn: 300 }`.
2. **Device displays the 6-digit `code`**, then polls
   **`GET /api/pair/status/[pairingId]`** every ~3s with
   `Authorization: Bearer <pollSecret>`. Returns `{ paired: false }` until claimed.
3. **User enters the code in the web app** at `/app/devices` (existing UI,
   unchanged — a Kobo code is indistinguishable from a PaperS3 code). Backend mints
   `sk_device_xxx`; the device's next poll returns
   `{ paired: true, token, userEmail }`. Agent writes the token to disk and uses it
   as `Authorization: Bearer sk_device_xxx` on every `/api/import/kobo` POST.

Backend reference (web repo): routes `src/routes/api/pair/{request,status/[pairingId]}/+server.ts`
and `src/routes/app/api/pair/claim/+server.ts`; logic `src/lib/server/pairing.ts`;
token gen `src/lib/server/tokens.ts`. Code TTL 300s, source of truth in TS
(`PAIRING_CODE_TTL_SEC`). pollSecret stored as SHA-256 hash; brute-force caps live
in the backend already (issues #286, #260).

---

## UI on stock Nickel — three tiers

- **Tier 1 — NickelDBus dialog/toast.** Toast ✅ confirmed. Dialog (`dlgConfirm*`
  methods: `dlgConfirmCreate`/`dlgConfirmSetTitle`/`dlgConfirmSetBody`/`dlgConfirmShow`)
  ✅ **CONFIRMED on hardware 2026-06-04** — native Nickel modal renders, title +
  body legible, X-dismiss (see Step 0a). Enough to **show the pairing code in a
  persistent modal** for v1. ⚠️ Font size is Nickel-native, not exposed by
  NickelDBus — for a bigger code, put it in `dlgConfirmSetTitle` (renders larger
  than the body), or use FBInk scaled text (Tier 2). `qndb` with NO args/method
  blocks waiting on a signal — always pass `-m <method>` or it hangs.
- **Tier 2 — FBInk** for a drawn status screen (logo + "Syncing…/Synced — N
  highlights"). ✅ **CONFIRMED on hardware 2026-06-04** (text + 6×-scaled big code
  rendered + refreshed the eink — see Step 0b). draws to the Kobo framebuffer
  directly (no InkView dependency). ⚠️ **Must build from `master`, NOT the v1.25.0
  release tag** — v1.25.0's device table predates the Libra Colour, so it
  misidentifies device id 390, picks the wrong (mxcfb) refresh ioctl, and every
  draw fails with `MXCFB_SEND_UPDATE_V1_NTX: Interrupted system call` (pixels land
  in fb memory but never display). `master` has the `Libra Colour (monza) [390]`
  entry and correctly drives the `hwtcon` backend. ⚠️ **Correction: the Libra
  Colour is MediaTek (Mark 13 / "Monza"), NOT sunxi** — earlier assumption wrong;
  fbink reports `isSunxi=0`, "Enabled MediaTek quirks". Build via the
  `arm-kobo-linux-gnueabihf` cross-toolchain (`make kobo`, no `MINIMAL=1` — that
  strips the fixed-cell fonts the CLI needs). Binary is dynamically linked against
  `libfbink.so` — ship both. ⚠️ **FBInk renders grayscale only on Kaleido colour
  devices** (README L149) — text+grayscale-logo dashboard is fine; no colour logo.
- **Tier 3 — local web UI** (Kobo-UNCaGED-style). Out of scope; v3 pipe-dream.

---

## Build order (one task per fresh session)

### Step 0 — Verify the two remaining on-hardware unknowns — ✅ DONE 2026-06-04 (PASS/PASS)

**Goal:** confirm before designing around them.

- **0a. NickelDBus dialog renders legibly. ✅ PASS.** `qndb -m dlgConfirmCreate` →
  `dlgConfirmSetTitle "Librito Pairing"` → `dlgConfirmSetBody "Pairing code:
482913"` → `dlgConfirmShow` rendered a clean native Nickel modal, title large,
  code legible, X-dismiss. → **pairing code ships as a Tier-1 dialog.**
- **0b. FBInk on 4.45.23697. ✅ PASS — with the master-build caveat above.** A
  `master` build correctly IDs the device (`Kobo Libra Colour (390 => Monza @ Mark
13)`, `hwtcon` backend, MediaTek quirks), and `fbink -c` / `fbink -p` / scaled
  `fbink -S 6` all drew + refreshed the panel (text + a 6×-scaled "482913" big
  code, both fully centered with `-m -M`). The v1.25.0 release tag FAILED (wrong
  refresh ioctl — see caveat). → **Tier-2 dashboard viable (grayscale).**

#### Step-2 probe — NickelDBus capability + WiFi control (hardware-verified 2026-06-06)

Ahead of building Step 2, probed `qndb`'s full API (`qndb -a`) and tested the
methods the pairing flow needs. **These are confirmed on the Libra Colour; design
around them.**

- **Dialog is fully scriptable for a live pairing flow — single-dialog UX works:**
  - `dlgConfirmShow` is **non-blocking** — `qndb` returns immediately, so a Go/poll
    loop keeps running while the dialog is up. (Reminder: `qndb` with NO `-m` hangs
    on a signal — always pass a method.)
  - A standing dialog **live-updates in place**: calling `dlgConfirmSetBody` /
    `dlgConfirmSetTitle` after `dlgConfirmShow` changes the visible text with no
    flicker/recreate. → one dialog can go "code 482 913" → "Paired ✓".
  - `dlgConfirmClose` **closes it programmatically** (no user tap). The flow can
    tear the dialog down itself on success.
  - `dlgConfirmAcceptReject <title> <body> <acceptText> <rejectText>` shows **two
    buttons**; the press is read via the `dlgConfirmResult <int>` signal —
    **reject=0, accept=1** (verified), and tapping auto-dismisses. → a "Cancel"
    button is detectable, so the user can abort pairing.
  - ⚠️ **Don't fire dialog calls back-to-back** — a rapid create→show→update→close
    burst raced and rendered nothing. Sequence with a small settle between calls.
- **WiFi IS agent-controllable — but only the silent path; the non-silent path is
  destructive:**
  - `wfmConnectWirelessSilently` **drives the connect path with no UI**: invoking it
    emits `wmTryingToConnect` then `wmNetworkConnected` (both verified firing). →
    the agent can request WiFi and **wait on the `wmNetworkConnected` signal**
    before talking to the network.
  - 🛑 **`wfmConnectWireless` (non-silent) is forbidden in the agent.** It pops a
    full-screen "looking for networks" → network-picker modal and **crashed Nickel
    into a reboot** on hardware. Never call it. Same for `pwrReboot` as a recovery
    step (a reboot watchdog cost us the dev SSH — dropbear isn't boot-persistent).
  - **Two distinct WiFi states matter:** _disabled_ (radio off — airplane mode /
    `nsForceWifi disable`) **cannot** be recovered by `wfmConnectWirelessSilently`;
    _enabled-but-disconnected_ (radio on, just not associated — Nickel's natural
    battery-timer drop) is the real-world state the agent faces. The silent connect
    drives `wmTryingToConnect`/`wmNetworkConnected` from the enabled state; a clean
    enabled→reassociate recovery test is still owed (do it silent-only, no
    non-silent fallback, no reboot watchdog).
  - Other present methods: `nsForceWifi <enable|disable>`, `wfmSetAirplaneMode`,
    plus signals `wmNetworkConnected` / `wmNetworkDisconnected` / `wmWifiEnabled`
    (fire on _change_, not queryable). `Kobo eReader.conf` is at
    `/mnt/onboard/.kobo/Kobo/Kobo eReader.conf` (note the nested `Kobo/`).
- **Device toolbox (verified absent):** no `jq`, no `uuidgen`, no `fbink` on a
  stock-modded device; `qndb`, `wpa_cli`, `iwconfig` present. → the Step-1/2 agent
  must be a **self-contained Go binary** (parse JSON, generate the UUID v4, do
  HTTP in-process) — a shell+`jq` pairing script is not viable.

#### Step-2 implementation — end-to-end pairing verified on hardware (2026-06-06)

The `pair` subcommand pairs a Libra Colour against local web with no computer
touching the device. Verified live: code shown → claimed at `/app/devices` →
token written → `devices.{type=kobo, model="Kobo Libra Colour"}` set. Then plain
`agent` (no `--token`) picks up the token file and dry-run reads 6 highlights.

HW-tuned / HW-corrected values:

- **`settleDelay = 150 ms`** between qndb dialog calls renders reliably (the
  AcceptReject→SetBody→Close sequence rendered cleanly). Not lowered further;
  150 ms is the shipped value.
- **Model detection:** `/mnt/onboard/.kobo/version` is **6 CSV fields**; field 6
  is a zero-padded UUID whose trailing decimal is the Kobo device id. Real dump:
  `N…,4.9.77,4.45.23697,4.9.77,4.9.77,00000000-0000-0000-0000-000000000390` →
  id `390` → `"Kobo Libra Colour"`. Parser pinned to this layout
  (`internal/pair/model.go`).
- 🛑 **WiFi signal correction — supersedes the Step-2 probe's claim that
  `wfmConnectWirelessSilently` "emits `wmNetworkConnected`".** That only holds
  from a _disconnected_ state. When WiFi is **already connected** (the common
  case — the user is on WiFi when they pair), the silent connect changes nothing
  and **`wmNetworkConnected` never fires**; a strict signal-wait times out, which
  made the first hardware run fail with "pairing cancelled" before any `/request`.
  NickelDBus 0.2.0 has **no connection-state query method** (introspected:
  only connect/disconnect _signals_, no getter). **Fix:** `WiFi.Connect` now
  nudges via the silent path, waits the window for `wmNetworkConnected` as a
  courtesy, then **proceeds regardless** — the `/request` HTTP call is the real
  connectivity oracle. qndb signal-wait exit code is the fire/timeout signal
  (rc 0 = fired, non-zero = timeout); stdout stays empty either way.
  - **Known follow-up:** with `Connect` always proceeding, the No-WiFi
    Retry/Cancel dialog is now unreachable on the real impl — a genuinely-offline
    device instead transport-errors at `/request` → `ReqTransient` → TTL-bounded
    backoff (bounded, not a spin, but no dedicated dialog). Wiring transport
    failure to a No-WiFi outcome is deferred (not blocking; the state-machine
    tests still cover the dialog logic via the fake).

#### Step-0 byproducts — the dev backbone (built en route, reuse for all later steps)

Getting on-hardware required solving shell access + a WiFi-drop problem. These are
now solved and are the standing dev loop:

- **SSH root shell — custom dropbear.** Stock Kobo 4.x has **no telnet** (the
  MobileRead wiki "enable telnet" steps don't apply — `inetd.conf`/`inittab` aren't
  present). No maintained drop-in SSH package covers the Libra Colour. Solution:
  cross-compiled **dropbear** (+`dropbearkey`) from `obynio/kobopatch-ssh`'s
  toolchain — the `obynio/kobo-toolchain:crosstools` Docker image is **arm64-native**
  (NOT x86; don't force `--platform linux/amd64` on Apple Silicon, it'll fail "does
  not provide platform"). Built `dropbear` modern (v2025.89, has ML-KEM/sntrup761).
  Packaged a **self-installing KoboRoot.tgz** (binaries → `/usr/local/bin`, udev
  rule on `loop0` → boot launcher) — telnet-free first install via the normal
  `.kobo/` drop. ⚠️ **This dropbear has NO `-E` flag** (the kobopatch-ssh repo's
  stock `on-boot.sh` uses `-E` → "Invalid option -E" → never starts; strip it).
  Auth: **passwordless for dev** (blank root pw + `dropbear -B`), home `/`. **MUST
  re-enable auth before Step 2 (token lands) and Step 5 (installer ships).**
- **No `sftp-server` on device → scp fails** ("`/usr/libexec/sftp-server: not
found`"). Transfer files via **`cat`-pipe over the SSH master**:
  `ssh -S $CM root@IP 'cat > /tmp/x' < localfile`. Reliable; sizes verified equal.
- **WiFi drops by design — `ForceWifiOn=true`.** Nickel powers the WiFi radio down
  on its **own timer** (battery), ignoring link traffic — a gateway-ping keepalive
  does NOT stop it (confirmed: master dropped mid-command with the ping loop
  running). Dev fix: set `ForceWifiOn=true` under `[DeveloperSettings]` in
  `/mnt/onboard/.kobo/Kobo/Kobo eReader.conf` (editable over USB; reboot to load).
  Held 60 s idle, zero traffic, no drop. ⚠️ **Dev-only** — costs battery. **This is
  the SAME constraint Step 3 must respect: WiFi is normally DOWN; the product agent
  syncs opportunistically on WiFi-up windows, never assumes a standing link.**
- **Persistent SSH master pattern** for the dev loop (fast, single auth):
  `ssh -M -S /tmp/kobo-ssh-master -o PubkeyAuthentication=no
-o PreferredAuthentications=password -o ServerAliveInterval=10 -fN root@IP`,
  then `ssh -S /tmp/kobo-ssh-master root@IP '<cmd>'`.

⚠️ **Not boot-persistent yet:** dropbear + the on-device wifi keepalive die on
reboot; re-run the NickelMenu "SSH open" item (calls
`/mnt/onboard/.adds/librito/ssh-open.sh`) after each boot. Wire dropbear into a
proper boot hook (udev rule is staged but the launcher needs the `-E`-free start
line) when convenient — low priority until the agent itself ships.

- **Output:** ✅ both unknowns PASS, this doc updated, dev backbone established.

### Step 1 — Agent core (read SQLite → POST) — **highest priority, mod-independent**

**Goal:** the actual sync, no UI.

- Read highlights from `/mnt/onboard/.kobo/KoboReader.sqlite` (Bookmark table;
  char-offset based — maps to the import endpoint's NULL-word-index path).
- Build the `/api/import/kobo` payload; dedup key is the Kobo `BookmarkID` →
  `source_uid` (backend dedups on `(book_id, source, source_uid)`).
- POST with the device token; honor server-owned soft-delete (don't resurrect
  web-trashed highlights — omit `deleted_at`, same contract as `processSync`).
- ⚠️ v1 is **highlights-only**. Kobo annotations are out of scope (the web-only
  `notes` invariant). Don't emit annotation counts the backend can't store.
- Language: shell + `sqlite3` + `curl` + `jq` (all present or bundleable on Kobo),
  OR a small static ARM binary if shell gets unwieldy. Decide in-session.
  → **Resolved: static Go binary.** `jq`/`uuidgen` are absent on device (Step-2
  probe), and the pure-core/impure-edge split wanted real tests — see CLAUDE.md.
- **Output:** a script that, given a token, syncs highlights end-to-end. Testable
  against the live endpoint with a real paired token.

### Step 2 — On-device pairing — ✅ DONE 2026-06-06 (hardware-verified)

**Goal:** unpaired device → holds a valid token, no computer needed.

**Status: shipped** (PR #9). The `internal/pair` package implements the 3-call
flow as a `pair` subcommand; `main.go` dispatches `os.Args[1]=="pair"` else sync.
Hardware-verified end-to-end on the Libra Colour: code → claim at `/app/devices`
→ `paired ✓` → token written → sync round-trip (idempotent), with
`devices.type=kobo` + `devices.model="Kobo Libra Colour"` (the agent detects the
model from `/mnt/onboard/.kobo/version` and sends `deviceType`/`deviceModel`).
The implementation notes + HW-tuned values live in the **Step-2 implementation**
section above; the design/plan are local-only under `docs/superpowers/`.

What deviated from the bullets below (all corrected in-flight):

- **WiFi gate** — the bounded `wmNetworkConnected` wait fails for an ALREADY-
  connected device (NickelDBus 0.2.0 fires that signal only on a state _change_,
  and has no connection-state query). `Connect` now nudges silently then proceeds;
  the HTTP `/request` is the real connectivity oracle. Silent-path-only invariant
  intact. See the WiFi correction in the Step-2 implementation section.
- **Open follow-ups (GitHub issues, Feature / area:pairing):** #3 flip the menu
  item to Unpair once paired (+ confirm modal); #4 route a request transport-
  failure to the No-WiFi dialog (now unreachable since `Connect` always proceeds).
- **Prereq still standing:** SSH hardening gates the first PRODUCTION-token run
  (Step 5). Dev verification used local web + throwaway tokens, so it didn't block.

- Implement the 3-call flow above as a **`pair` subcommand of the Go agent**
  (same binary, not a shell script — no `jq`/`uuidgen` on device; see the Step-2
  probe), triggered by a NickelMenu item ("Pair Librito") via a thin launcher.
- Generate+persist `hardwareId` once: **lowercase canonical UUID v4** from
  `crypto/rand`. The web `hardware_id` UNIQUE is case-sensitive text — a
  mixed-case resend would create a phantom second device. Persist to
  `/mnt/onboard/.adds/librito/hardware-id`.
- **Wire contract (verified against web):** `POST /api/pair/request {hardwareId}`
  → `{code, pairingId, expiresIn:300, pollSecret}`. The `pollSecret` is returned
  ONCE and **must** be sent as `Authorization: Bearer <pollSecret>` on every
  status poll. `GET /api/pair/status/[pairingId]` → `{paired:false}` until the
  user enters the code at `/app/devices`, then `{paired:true, token, userEmail}`.
  Poll at ~5 s (server caps 1/3 s, fails open); code TTL 300 s; on `410` the code
  expired → request a fresh one. The PaperS3 firmware (`src/cloud/PairingTask.cpp`)
  is the working reference for this exact flow.
- Display via the Step-2-verified **single live-updating NickelDBus dialog**: show
  the code, live-update to "Paired ✓" on success, auto-close; a Cancel button
  (`dlgConfirmAcceptReject` + `dlgConfirmResult`) aborts. On `paired:true` write
  the token to `/mnt/onboard/.adds/librito/token`.
- **WiFi:** before the first request, call `wfmConnectWirelessSilently` and wait
  (bounded) for `wmNetworkConnected`; tolerate mid-poll drops by re-nudging
  silently. **Never** `wfmConnectWireless` (non-silent) or `pwrReboot` — both are
  destructive (see Step-2 probe).
- **Prereq:** re-enable SSH auth (dropbear is passwordless for dev) before a real
  token lands — do it before the first end-to-end on-device pairing run.
- **Output:** tap-to-pair working on-device against the existing web flow.

### Step 3 — udev WiFi-up auto-sync trigger — **the robust backbone** ❓

**Goal:** highlights sync automatically when the Kobo gets WiFi, no manual tap.

- This is kernel/userland (udev rule on the WiFi interface coming up), **NOT a
  Nickel hook** — deliberately independent of NickelMenu/NickelDBus so the backbone
  survives mod breakage. ❓ entirely unverified — feasibility unknown, next big
  investigation after Steps 1–2.
- Fallback if udev proves unworkable: NickelDBus `wmStarted`/wifi signal
  (`qndb -s …`) as the trigger — but that re-introduces the mod dependency, so
  udev is preferred.
- **Output:** WiFi-up reliably fires the Step-1 agent.

### Step 4 — FBInk status dashboard (polish)

**Goal:** the "native app feel" screen — logo, syncing/synced, N highlights.

- Only if Step 0b PASS. Grayscale. Launched from a NickelMenu item; agent writes
  status, FBInk renders.
- **Output:** one-page status dashboard.

### Step 5 — Mac installer app (widens userbase past power-users)

**Goal:** one-file, drag-free setup.

- Native SwiftUI (Mac-only for now). Detects mounted Kobo (DiskArbitration),
  reads/pre-flights firmware (warn if newer than tested), copies a **single bundled
  `KoboRoot.tgz`** (NickelMenu + NickelDBus + FBInk + agent + pre-written NM config)
  into `.kobo/`, ejects, shows "now reboot + pair" guidance.
- Auth stays on-device (Step 2) + web app — the Mac app is **just the installer**,
  not the token carrier. This keeps the Instapaper/Storygraph-like ceremony
  (pair-on-device, confirm-in-web) intact.
- ⚠️ Bundling someone else's stale NDB binary means you inherit its
  firmware-fragility — the firmware pre-flight check is the mitigation/trust
  feature.
- **Output:** a Mac app that turns setup into "install app → plug in → click Set
  Up → pair".

---

## Repos / orientation

- **`librito-io/kobo-agent`** — the agent + this doc live here.
- **`librito-io/reader`** — PaperS3 firmware; source of the `parseIsbnFromOpf`
  algorithm to port (web#503).
- **`librito-io/web`** — import endpoint + pairing flow, already shipped; **no
  changes needed** for Steps 0–4. Step 5 Mac app talks to the same web endpoints.
