// Package status holds the PURE display decisions for the on-device surface
// (agent status / about / sync-now). All functions are total + table-tested; the
// device edges (file read, qndb) live in autosync/main. Imports autosync for its
// Record + Outcome types (one-way; autosync never imports status).
package status

import (
	"fmt"
	"time"

	"github.com/librito-io/kobo-agent/internal/autosync"
)

// DecideStatusLine renders the single `agent status` line. Precedence is PINNED
// (first match wins) — see the spec table. A prior success is the durable truth:
// once it has ever synced, show WHEN and let a stale value be the self-revealing
// alarm (no alarmist "failed" line after a real success).
func DecideStatusLine(rec autosync.Record, now time.Time, hasToken bool) string {
	switch {
	case !hasToken:
		return "Not paired"
	case rec.LastSuccessAt != nil:
		return "Last synced " + FormatRelative(now.Sub(*rec.LastSuccessAt))
	case rec.LastOutcome == "offline":
		return "Waiting for WiFi"
	case rec.LastOutcome == "error":
		return "Sync isn't working yet — tap Sync now"
	default:
		return "Not synced yet"
	}
}

// FormatRelative renders a coarse, honest "X ago". Coarse on purpose: the wall
// clock is only best-effort (spec "Time on the Kobo"), so sub-minute precision
// would be false confidence.
func FormatRelative(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour")
	default:
		return plural(int(d/(24*time.Hour)), "day")
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}

// DecideSyncResult maps autosync.Run's returned Outcome to the sync-now dialog
// line. Takes the Outcome directly (a plain enum) — it never reads the record file.
func DecideSyncResult(o autosync.Outcome) string {
	switch o {
	case autosync.OutcomeSynced:
		return "Highlights synced ✓"
	case autosync.OutcomeOffline:
		return "Please connect to WiFi to sync highlights to Librito"
	case autosync.OutcomeDedup:
		return "A sync is already running — your highlights are on their way"
	case autosync.OutcomeUnpaired:
		return "Not paired"
	default: // OutcomeError, OutcomeLockErr
		return "Couldn't reach Librito right now"
	}
}

// AboutLines builds the `agent about` output. Pure so the token gating, the
// RFC3339 parse→"Jan 2, 2006" format, and the line-omission rules are
// table-tested — the main.go glue is then just file reads + printing. An empty
// or unparseable pairedAt drops the "Paired" line rather than showing a bogus
// date; an empty email is omitted. Unpaired short-circuits to a single line.
func AboutLines(hasToken bool, email, pairedAt, version string) []string {
	if !hasToken {
		return []string{"Not paired"}
	}
	var lines []string
	if email != "" {
		lines = append(lines, email)
	}
	if t, err := time.Parse(time.RFC3339, pairedAt); err == nil {
		lines = append(lines, "Paired "+t.Format("Jan 2, 2006"))
	}
	return append(lines, "Librito agent "+version)
}
