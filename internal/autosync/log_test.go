package autosync

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatLine(t *testing.T) {
	ts := time.Date(2026, 6, 7, 19, 42, 0, 0, time.UTC)
	cases := []struct {
		name string
		t    time.Time
		tag  string
		msg  string
		want string
	}{
		{
			"success",
			ts,
			"autosync",
			"imported 6 across 6 books",
			"2026-06-07T19:42:00Z autosync: imported 6 across 6 books\n",
		},
		{
			"error",
			ts,
			"autosync",
			"post import: dial tcp: timeout",
			"2026-06-07T19:42:00Z autosync: post import: dial tcp: timeout\n",
		},
		{
			"non-utc input is converted to UTC",
			time.Date(2026, 6, 7, 20, 42, 0, 0, time.FixedZone("BST", 3600)),
			"autosync",
			"skipped: not paired",
			"2026-06-07T19:42:00Z autosync: skipped: not paired\n",
		},
		{
			"watch tag",
			ts,
			"watch",
			"signature grew 7→8",
			"2026-06-07T19:42:00Z watch: signature grew 7→8\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatLine(c.t, c.tag, c.msg); got != c.want {
				t.Fatalf("FormatLine = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCapLog(t *testing.T) {
	small := []byte("line1\nline2\n")
	if got := capLog(small, 64*1024); string(got) != string(small) {
		t.Fatalf("under-cap content was modified: %q", got)
	}

	// Oversized: keep the newest lines, never start mid-line.
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("0123456789abcdef\n") // 17 bytes/line
	}
	capped := capLog([]byte(b.String()), 100)
	if len(capped) > 100 {
		t.Fatalf("capped length %d > max 100", len(capped))
	}
	if !strings.HasSuffix(string(capped), "0123456789abcdef\n") {
		t.Fatalf("capped content should end with a whole line, got %q", capped)
	}
	if strings.Count(string(capped), "\n") != strings.Count(string(capped), "0123456789abcdef\n") {
		t.Fatalf("capped content starts mid-line: %q", capped)
	}
}

func TestFileLogger_AppendsAndCaps(t *testing.T) {
	path := t.TempDir() + "/autosync.log"
	l := NewFileLogger(path, 200)

	for i := 0; i < 50; i++ {
		l.Log(FormatLine(time.Date(2026, 6, 7, 19, 42, i%60, 0, time.UTC), "autosync", "imported 6 across 6 books"))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 200 {
		t.Fatalf("log size %d exceeds cap 200", len(b))
	}
	if !strings.HasPrefix(string(b), "2026-") {
		t.Fatalf("log starts mid-line: %q", b)
	}
	if !strings.Contains(string(b), "autosync: imported 6 across 6 books") {
		t.Fatalf("log missing the newest line: %q", b)
	}
}
