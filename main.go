// Command librito-kobo-sync reads highlights from a Kobo's KoboReader.sqlite
// and syncs them to the Librito import endpoint (POST /api/import/kobo); the
// `pair` subcommand obtains a device token via the Librito pairing API.
//
// Run `librito-kobo-sync --help` for the list of subcommands. With no
// subcommand it performs a sync.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/librito-io/kobo-sync/internal/autosync"
	"github.com/librito-io/kobo-sync/internal/pair"
	"github.com/librito-io/kobo-sync/internal/status"
	"github.com/librito-io/kobo-sync/internal/sync"
	"github.com/librito-io/kobo-sync/internal/watch"
)

// adsDir is where pairing persists hardware-id + token (co-located with the
// binary on-device).
const adsDir = "/mnt/onboard/.adds/librito"

// version is the agent version, set at build time via -ldflags -X main.version.
// The single source of truth surfaced by `kobo-sync about`.
var version = "dev"

func main() {
	// Display the program as it was actually invoked (on-device the binary is
	// installed as "kobo-sync", not the dev artifact name), so help + error
	// pointers name a command that exists on the target. progName is the fallback.
	prog := progName
	if b := filepath.Base(os.Args[0]); b != "" && b != "." {
		prog = b
	}
	os.Exit(dispatch(os.Args[1:], prog, commands, runSync, os.Stdout, os.Stderr))
}

func runPair(argv []string) int {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	baseURL := fs.String("url", "https://librito.io", "Librito API base URL")
	dir := fs.String("dir", adsDir, "directory for hardware-id + token files")
	model := fs.String("model", "", "override the device model sent on pairing (default: detect from the Kobo version file)")
	versionPath := fs.String("version-file", pair.DefaultVersionPath, "path to the Kobo version file used to detect the model")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}

	// Model precedence: --model override > detected from the version file >
	// legible fallback (DetectModel never errors; it degrades to "Kobo").
	deviceModel := *model
	if deviceModel == "" {
		deviceModel = pair.DetectModel(*versionPath)
	}

	res := pair.Run(pair.Deps{
		Client:         pair.NewHTTPClient(*baseURL, 30*time.Second),
		Display:        pair.NewQndbDisplay(),
		WiFi:           pair.NewQndbWiFi(),
		Store:          pair.NewFileStore(*dir, rand.Reader),
		Clock:          realClock{},
		WallNow:        time.Now,
		DeviceModel:    deviceModel,
		BaseURL:        *baseURL,
		WiFiTimeout:    20 * time.Second,
		PollEvery:      5 * time.Second,
		CodeTTL:        300 * time.Second,
		DecisionWindow: 120 * time.Second, // time to tap Retry/Cancel on a No-WiFi / expired prompt
	})

	// Exit codes are a stable signal for a NickelMenu / udev wrapper: 0 ONLY when
	// a token was written. Cancelled / expired / fatal are all distinct nonzero so
	// a wrapper never mistakes "no token" for "paired".
	switch res.Status {
	case pair.ResultPaired:
		fmt.Println("paired ✓")
		return 0
	case pair.ResultCancelled:
		fmt.Fprintln(os.Stderr, "pairing cancelled")
		return 3
	case pair.ResultExpired:
		fmt.Fprintln(os.Stderr, "pairing code expired")
		return 1
	default: // ResultFatal
		fmt.Fprintln(os.Stderr, "pairing failed")
		return 1
	}
}

func runSync(argv []string) int {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	dbPath := fs.String("db", "/mnt/onboard/.kobo/KoboReader.sqlite", "path to KoboReader.sqlite")
	baseURL := fs.String("url", "https://librito.io", "Librito API base URL")
	token := fs.String("token", "", "device token (sk_device_...); or set LIBRITO_TOKEN, or pair first")
	dir := fs.String("dir", adsDir, "directory holding the paired token file")
	dryRun := fs.Bool("dry-run", false, "read + map + report, do not POST")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}

	tok := resolveToken(*token, os.Getenv("LIBRITO_TOKEN"), filepath.Join(*dir, "token"))
	if tok == "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "error: no device token (--token, LIBRITO_TOKEN, or pair first); use --dry-run to inspect without one")
		return 2
	}

	out, err := sync.Run(sync.Options{DBPath: *dbPath, BaseURL: *baseURL, Token: tok, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
		return 1
	}

	if *dryRun {
		fmt.Printf("dry-run: read %d highlights, built %d items (no POST)\n", out.Read, out.Built)
		for _, it := range out.Items {
			isbn := "—"
			if it.ISBN != nil {
				isbn = *it.ISBN
			}
			fmt.Printf("  [%s] %q isbn=%s\n", it.Title, truncate(it.Text, 50), isbn)
		}
		return 0
	}

	fmt.Printf("synced: read %d, sent %d → server imported %d across %d books\n",
		out.Read, out.Built, out.Result.Imported, out.Result.Books)
	return 0
}

func runAutosync(argv []string) int {
	fs := flag.NewFlagSet("autosync", flag.ExitOnError)
	dbPath := fs.String("db", "/mnt/onboard/.kobo/KoboReader.sqlite", "path to KoboReader.sqlite")
	dir := fs.String("dir", adsDir, "directory holding the token + url files")
	defaultURL := fs.String("url", "https://librito.io", "fallback API base URL when no url file is present (pairing writes the url file)")
	lockPath := fs.String("lock", "/tmp/librito-autosync.lock", "single-instance lock path (tmpfs)")
	logPath := fs.String("log", filepath.Join(adsDir, "autosync.log"), "append-only result log path")
	recordPath := fs.String("record", "", "last-sync record path (default: <dir>/last-sync)")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}
	*recordPath = defaultRecordPath(*recordPath, *dir)

	return autosync.Run(autosync.Deps{
		Locker:     autosync.NewFlockLocker(*lockPath),
		Config:     autosync.NewFileConfig(*dir, *defaultURL),
		Prober:     autosync.NewSysfsProber("wlan0"),
		Syncer:     autosync.NewSyncer(*dbPath),
		Logger:     autosync.NewFileLogger(*logPath, 64*1024),
		Clock:      realClock{},
		Record:     autosync.NewFileRecordStore(*recordPath, time.Now),
		ViewProber: autosync.NewQndbViewProber(),
		Toaster:    autosync.NewQndbToaster(4000),
		Timeout:    60 * time.Second,
		Cadence:    2 * time.Second,
	}).ExitCode()
}

// runWatch runs the resident watcher daemon (or, with --probe, the inotify spike).
func runWatch(argv []string) int {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	dbPath := fs.String("db", "/mnt/onboard/.kobo/KoboReader.sqlite", "path to KoboReader.sqlite (its directory is watched)")
	dir := fs.String("dir", adsDir, "directory holding the token + url files")
	defaultURL := fs.String("url", "https://librito.io", "fallback API base URL when no url file is present")
	syncLock := fs.String("lock", "/tmp/librito-autosync.lock", "shared sync lock (serializes against the udev autosync)")
	watchLock := fs.String("watch-lock", "/tmp/librito-watch.lock", "single-instance lock for this daemon")
	logPath := fs.String("log", filepath.Join(adsDir, "autosync.log"), "append-only result log path (shared with autosync)")
	recordPath := fs.String("record", "", "last-sync record path (default: <dir>/last-sync; MUST match the autosync run's — the record holds the toast growth baseline)")
	walName := fs.String("wal-name", "", "WAL filename to react to (default: <db basename>-wal; escape hatch if the spike shows a different name)")
	probe := fs.Bool("probe", false, "log raw inotify events and run until killed (hardware spike)")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}
	*recordPath = defaultRecordPath(*recordPath, *dir)

	wal := *walName
	if wal == "" {
		wal = filepath.Base(*dbPath) + "-wal"
	}

	watchDir := filepath.Dir(*dbPath) // the watched directory (holds KoboReader.sqlite + its -wal)
	w, err := watch.NewWatcher(watchDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: %v\n", err)
		return 1
	}
	defer func() { _ = w.Close() }()

	if *probe {
		fmt.Printf("watch --probe: watching %s — make a highlight, Ctrl-C to stop\n", watchDir)
		for ev := range w.Events() {
			fmt.Printf("event: name=%q mask=%#x\n", ev.Name, ev.Mask)
		}
		return 0
	}

	// Stateless edges shared by both Deps structs (one instance serves both).
	prober := autosync.NewSysfsProber("wlan0")
	logger := autosync.NewFileLogger(*logPath, 64*1024)

	// Resident daemon. The sync delegates to autosync.Run with the SAME shared
	// lock as the udev path (so they never double-run) but a SHORT timeout (we
	// only sync when already connected). Single-instance via a SEPARATE watch lock.
	// ViewProber/Toaster ARE wired (like the udev path): whichever run wins the
	// shared lock performs the real import, so it must own the toast — otherwise a
	// highlight captured while connected, imported by the watch winner, leaves the
	// udev loser to dedup and toast nothing. The growth gate keeps this quiet: the
	// toast fires only when the set grew AND the view is allow-listed, so an in-book
	// capture (ReadingView) never toasts; the only added cost is one qndb view-probe
	// per real growth (skipped entirely when nothing grew). qndb is absolute, so the
	// probe works regardless of how the daemon was launched.
	runner := watch.NewRunner(autosync.Deps{
		Locker:     autosync.NewFlockLocker(*syncLock),
		Config:     autosync.NewFileConfig(*dir, *defaultURL),
		Prober:     prober,
		Syncer:     autosync.NewSyncer(*dbPath),
		Logger:     logger,
		Clock:      realClock{},
		Record:     autosync.NewFileRecordStore(*recordPath, time.Now),
		ViewProber: autosync.NewQndbViewProber(),
		Toaster:    autosync.NewQndbToaster(4000),
		Timeout:    5 * time.Second,
		Cadence:    2 * time.Second,
	})

	return watch.Run(watch.Deps{
		Locker:    autosync.NewFlockLocker(*watchLock),
		Watcher:   w,
		SigReader: watch.NewSigReader(*dbPath),
		Prober:    prober,
		Runner:    runner,
		Logger:    logger,
		Clock:     realClock{},
		WALName:   wal,
		Window:    5 * time.Second,
		Cap:       15 * time.Second,
	})
}

func runStatus(argv []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dir := fs.String("dir", adsDir, "directory holding the token + last-sync files")
	recordPath := fs.String("record", "", "last-sync record path (default: <dir>/last-sync)")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}
	*recordPath = defaultRecordPath(*recordPath, *dir)

	hasToken := resolveToken("", "", filepath.Join(*dir, "token")) != ""
	rec, _ := autosync.LoadRecord(*recordPath)
	fmt.Println(status.DecideStatusLine(rec, time.Now(), hasToken))
	return 0
}

func runAbout(argv []string) int {
	fs := flag.NewFlagSet("about", flag.ExitOnError)
	dir := fs.String("dir", adsDir, "directory holding the account files")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}

	hasToken := resolveToken("", "", filepath.Join(*dir, "token")) != ""
	lines := status.AboutLines(hasToken,
		readTrim(filepath.Join(*dir, "email")),
		readTrim(filepath.Join(*dir, "paired-at")),
		version)
	for _, l := range lines {
		fmt.Println(l)
	}
	return 0
}

func runSyncNow(argv []string) int {
	fs := flag.NewFlagSet("sync-now", flag.ExitOnError)
	dbPath := fs.String("db", "/mnt/onboard/.kobo/KoboReader.sqlite", "path to KoboReader.sqlite")
	dir := fs.String("dir", adsDir, "directory holding the token + url files")
	defaultURL := fs.String("url", "https://librito.io", "fallback API base URL")
	lockPath := fs.String("lock", "/tmp/librito-autosync.lock", "shared sync lock")
	logPath := fs.String("log", filepath.Join(adsDir, "autosync.log"), "result log path")
	recordPath := fs.String("record", "", "last-sync record path (default: <dir>/last-sync)")
	_ = fs.Parse(argv)
	if rejectPositionals(fs) {
		return 2
	}
	*recordPath = defaultRecordPath(*recordPath, *dir)

	// 1. Best-effort "Syncing…" toast fills the cmd_output dead-gap (menu closes,
	//    nothing on screen until we print). Separate from the in-Run success toast.
	autosync.NewQndbToaster(4000).Toast("Syncing…", "")

	// 2. Delegate to the shared engine with a SHORT timeout (no WiFi bring-up;
	//    its own WaitForConnectivity fast-fails offline). No-op View/Toaster so the
	//    in-Run success toast is suppressed — sync-now shows the result dialog instead.
	out := autosync.Run(autosync.Deps{
		Locker:     autosync.NewFlockLocker(*lockPath),
		Config:     autosync.NewFileConfig(*dir, *defaultURL),
		Prober:     autosync.NewSysfsProber("wlan0"),
		Syncer:     autosync.NewSyncer(*dbPath),
		Logger:     autosync.NewFileLogger(*logPath, 64*1024),
		Clock:      realClock{},
		Record:     autosync.NewFileRecordStore(*recordPath, time.Now),
		ViewProber: autosync.NewNoopViewProber(),
		Toaster:    autosync.NewNoopToaster(),
		Timeout:    5 * time.Second,
		Cadence:    2 * time.Second,
	})

	// 3. Map the RETURNED outcome (never re-read the record) → result dialog text.
	fmt.Println(status.DecideSyncResult(out))
	return out.ExitCode()
}

// defaultRecordPath resolves a --record flag value: an explicit path wins, else
// <dir>/last-sync. SINGLE ORIGIN for all four record-touching subcommands
// (autosync, status, sync-now, watch) — the record carries the toast growth
// baseline, so a drifted derivation in one handler would split the baseline
// between the udev autosync and the watch daemon (#38).
func defaultRecordPath(record, dir string) string {
	if record != "" {
		return record
	}
	return filepath.Join(dir, "last-sync")
}

// readTrim reads a small file, returning its trimmed contents or "".
func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// resolveToken applies the precedence: flag > env > token file. A read error or
// missing file falls through to "".
func resolveToken(flagTok, envTok, tokenFile string) string {
	if flagTok != "" {
		return flagTok
	}
	if envTok != "" {
		return envTok
	}
	if b, err := os.ReadFile(tokenFile); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}

// realClock is the production Clock: monotonic Now + real Sleep.
type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
