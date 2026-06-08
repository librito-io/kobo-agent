package status

import (
	"testing"
	"time"

	"github.com/librito-io/kobo-agent/internal/autosync"
)

func ptr(s string) *time.Time { t, _ := time.Parse(time.RFC3339, s); return &t }

func TestDecideStatusLine(t *testing.T) {
	now := mustParse("2026-06-08T16:44:00Z")
	cases := []struct {
		name     string
		rec      autosync.Record
		hasToken bool
		want     string
	}{
		{"not paired wins", autosync.Record{LastSuccessAt: ptr("2026-06-08T16:42:00Z")}, false, "Not paired"},
		{"prior success → relative", autosync.Record{LastSuccessAt: ptr("2026-06-08T16:42:00Z"), LastOutcome: "ok"}, true, "Last synced 2 minutes ago"},
		{"prior success wins over later offline", autosync.Record{LastSuccessAt: ptr("2026-06-08T16:42:00Z"), LastOutcome: "offline"}, true, "Last synced 2 minutes ago"},
		{"never synced, offline attempt", autosync.Record{LastOutcome: "offline"}, true, "Waiting for WiFi"},
		{"never synced, error attempt", autosync.Record{LastOutcome: "error"}, true, "Sync isn't working yet — tap Sync now"},
		{"no record at all", autosync.Record{}, true, "Not synced yet"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DecideStatusLine(c.rec, now, c.hasToken); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestFormatRelative(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-1 * time.Second, "just now"}, // clock skew / future stamp → safe floor
		{20 * time.Second, "just now"},
		{1 * time.Minute, "1 minute ago"},
		{2 * time.Minute, "2 minutes ago"},
		{90 * time.Minute, "1 hour ago"},
		{25 * time.Hour, "1 day ago"},
		{73 * time.Hour, "3 days ago"},
	}
	for _, c := range cases {
		if got := FormatRelative(c.d); got != c.want {
			t.Errorf("FormatRelative(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestDecideSyncResult(t *testing.T) {
	cases := map[autosync.Outcome]string{
		autosync.OutcomeSynced:   "Highlights synced ✓",
		autosync.OutcomeOffline:  "Please connect to WiFi to sync highlights to Librito",
		autosync.OutcomeError:    "Couldn't reach Librito right now",
		autosync.OutcomeLockErr:  "Couldn't reach Librito right now",
		autosync.OutcomeDedup:    "A sync is already running — your highlights are on their way",
		autosync.OutcomeUnpaired: "Not paired",
	}
	for o, want := range cases {
		if got := DecideSyncResult(o); got != want {
			t.Errorf("DecideSyncResult(%d) = %q, want %q", o, got, want)
		}
	}
}

func mustParse(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }
