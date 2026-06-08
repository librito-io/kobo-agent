// Command librito-kobo-agent reads highlights from a Kobo's KoboReader.sqlite
// and syncs them to the Librito import endpoint (POST /api/import/kobo); the
// `pair` subcommand obtains a device token via the Librito pairing API.
//
// Usage:
//
//	librito-kobo-agent              sync (token from --token / LIBRITO_TOKEN / token file)
//	librito-kobo-agent pair         pair this device (writes hardware-id + token)
//	librito-kobo-agent autosync     triggered sync (udev WiFi-up); token + url from files
//	librito-kobo-agent watch        resident daemon: immediate sync on a new highlight while connected
//	librito-kobo-agent status       print the last-sync status line
//	librito-kobo-agent about        print pairing info + agent version
//	librito-kobo-agent sync-now     run a one-shot sync with feedback (for NickelMenu)
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/librito-io/kobo-agent/internal/autosync"
	"github.com/librito-io/kobo-agent/internal/pair"
	"github.com/librito-io/kobo-agent/internal/status"
	"github.com/librito-io/kobo-agent/internal/sync"
	"github.com/librito-io/kobo-agent/internal/watch"
)

// adsDir is where pairing persists hardware-id + token (co-located with the
// binary on-device).
const adsDir = "/mnt/onboard/.adds/librito"

// version is the agent version, set at build time via -ldflags -X main.version.
// The single source of truth surfaced by `agent about`.
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "pair" {
		os.Exit(runPair(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "autosync" {
		os.Exit(runAutosync(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "watch" {
		os.Exit(runWatch(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "status" {
		os.Exit(runStatus(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "about" {
		os.Exit(runAbout(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "sync-now" {
		os.Exit(runSyncNow(os.Args[2:]))
	}
	os.Exit(runSync(os.Args[1:]))
}

func runPair(argv []string) int {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	baseURL := fs.String("url", "https://librito.io", "Librito API base URL")
	dir := fs.String("dir", adsDir, "directory for hardware-id + token files")
	model := fs.String("model", "", "override the device model sent on pairing (default: detect from the Kobo version file)")
	versionPath := fs.String("version-file", pair.DefaultVersionPath, "path to the Kobo version file used to detect the model")
	_ = fs.Parse(argv)

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
	recordPath := fs.String("record", filepath.Join(adsDir, "last-sync"), "last-sync record path")
	_ = fs.Parse(argv)

	return autosync.Run(autosync.Deps{
		Locker:     autosync.NewFlockLocker(*lockPath),
		Config:     autosync.NewFileConfig(*dir, *defaultURL),
		Prober:     autosync.NewSysfsProber("wlan0"),
		Syncer:     autosync.NewSyncer(*dbPath),
		Logger:     autosync.NewFileLogger(*logPath, 64*1024),
		Clock:      realClock{},
		Record:     autosync.NewFileRecordStore(*recordPath, time.Now),
		ViewProber: autosync.NewQndbViewProber(),
		Toaster:    autosync.NewQndbToaster("2000"),
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
	walName := fs.String("wal-name", "", "WAL filename to react to (default: <db basename>-wal; escape hatch if the spike shows a different name)")
	probe := fs.Bool("probe", false, "log raw inotify events and run until killed (hardware spike)")
	_ = fs.Parse(argv)

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
	// Record is wired (watch tracks last-sync) but ViewProber/Toaster are nil —
	// suppressing the toast for the highlight-while-reading path is correct and
	// avoids a qndb call per highlight.
	runner := watch.NewRunner(autosync.Deps{
		Locker:  autosync.NewFlockLocker(*syncLock),
		Config:  autosync.NewFileConfig(*dir, *defaultURL),
		Prober:  prober,
		Syncer:  autosync.NewSyncer(*dbPath),
		Logger:  logger,
		Clock:   realClock{},
		Record:  autosync.NewFileRecordStore(filepath.Join(*dir, "last-sync"), time.Now),
		Timeout: 5 * time.Second,
		Cadence: 2 * time.Second,
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
	recordPath := fs.String("record", filepath.Join(adsDir, "last-sync"), "last-sync record path")
	_ = fs.Parse(argv)

	hasToken := resolveToken("", "", filepath.Join(*dir, "token")) != ""
	rec, _ := autosync.LoadRecord(*recordPath)
	fmt.Println(status.DecideStatusLine(rec, time.Now(), hasToken))
	return 0
}

func runAbout(argv []string) int {
	fs := flag.NewFlagSet("about", flag.ExitOnError)
	dir := fs.String("dir", adsDir, "directory holding the account files")
	_ = fs.Parse(argv)

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
	recordPath := fs.String("record", filepath.Join(adsDir, "last-sync"), "last-sync record path")
	_ = fs.Parse(argv)

	// 1. Best-effort "Syncing…" toast fills the cmd_output dead-gap (menu closes,
	//    nothing on screen until we print). Separate from the in-Run success toast.
	autosync.NewQndbToaster("4000").Toast("Syncing…", "")

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
