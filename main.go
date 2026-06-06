// Command librito-kobo-agent reads highlights from a Kobo's KoboReader.sqlite
// and syncs them to the Librito import endpoint (POST /api/import/kobo); the
// `pair` subcommand obtains a device token via the Librito pairing API.
//
// Usage:
//
//	librito-kobo-agent              sync (token from --token / LIBRITO_TOKEN / token file)
//	librito-kobo-agent pair         pair this device (writes hardware-id + token)
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/librito-io/kobo-agent/internal/pair"
	"github.com/librito-io/kobo-agent/internal/sync"
)

// adsDir is where pairing persists hardware-id + token (co-located with the
// binary on-device).
const adsDir = "/mnt/onboard/.adds/librito"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "pair" {
		os.Exit(runPair(os.Args[2:]))
	}
	os.Exit(runSync(os.Args[1:]))
}

func runPair(argv []string) int {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	baseURL := fs.String("url", "https://librito.io", "Librito API base URL")
	dir := fs.String("dir", adsDir, "directory for hardware-id + token files")
	_ = fs.Parse(argv)

	res := pair.Run(pair.Deps{
		Client:         pair.NewHTTPClient(*baseURL, 30*time.Second),
		Display:        pair.NewQndbDisplay(),
		WiFi:           pair.NewQndbWiFi(),
		Store:          pair.NewFileStore(*dir, rand.Reader),
		Clock:          realClock{},
		WiFiTimeout:    20 * time.Second,
		PollEvery:      5 * time.Second,
		CodeTTL:        300 * time.Second,
		DecisionWindow: 120 * time.Second, // time to tap Retry/Cancel on a No-WiFi / expired prompt
	})

	switch res.Status {
	case pair.ResultPaired:
		fmt.Println("paired ✓")
		return 0
	case pair.ResultCancelled:
		fmt.Fprintln(os.Stderr, "pairing cancelled")
		return 0
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
