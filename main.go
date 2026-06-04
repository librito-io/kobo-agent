// Command librito-kobo-agent reads highlights from a Kobo's KoboReader.sqlite
// and syncs them to the Librito import endpoint (POST /api/import/kobo).
//
// Step 1 of the Kobo agent: the mod-independent sync core. No UI, no pairing —
// it takes a device token and a DB path and does the round-trip. Pairing
// (Step 2) writes the token; a WiFi-up trigger (Step 3) invokes this.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/librito-io/kobo-agent/internal/sync"
)

func main() {
	var (
		dbPath  = flag.String("db", "/mnt/onboard/.kobo/KoboReader.sqlite", "path to KoboReader.sqlite")
		baseURL = flag.String("url", "https://librito.io", "Librito API base URL")
		token   = flag.String("token", "", "device token (sk_device_...); or set LIBRITO_TOKEN")
		dryRun  = flag.Bool("dry-run", false, "read + map + report, do not POST")
	)
	flag.Parse()

	tok := *token
	if tok == "" {
		tok = os.Getenv("LIBRITO_TOKEN")
	}
	if tok == "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "error: no device token (--token or LIBRITO_TOKEN); use --dry-run to inspect without one")
		os.Exit(2)
	}

	out, err := sync.Run(sync.Options{
		DBPath:  *dbPath,
		BaseURL: *baseURL,
		Token:   tok,
		DryRun:  *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
		os.Exit(1)
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
		return
	}

	fmt.Printf("synced: read %d, sent %d → server imported %d across %d books\n",
		out.Read, out.Built, out.Result.Imported, out.Result.Books)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
