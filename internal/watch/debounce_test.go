package watch

import (
	"testing"
	"time"
)

func TestDebounceWait(t *testing.T) {
	base := time.Date(2026, 6, 7, 19, 42, 0, 0, time.UTC)
	const window = 5 * time.Second
	const maxWait = 15 * time.Second

	cases := []struct {
		name             string
		first, last, now time.Time
		want             time.Duration
	}{
		{"fresh event waits the full window", base, base, base, 5 * time.Second},
		{"silence partially elapsed", base, base.Add(2 * time.Second), base.Add(3 * time.Second), 4 * time.Second},
		{"window elapsed → due now", base, base, base.Add(5 * time.Second), 0},
		{"cap bounds a chatty stream", base, base.Add(14 * time.Second), base.Add(14 * time.Second), 1 * time.Second},
		{"cap elapsed → due (negative)", base, base.Add(20 * time.Second), base.Add(20 * time.Second), -5 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := debounceWait(c.first, c.last, c.now, window, maxWait); got != c.want {
				t.Fatalf("debounceWait = %v, want %v", got, c.want)
			}
		})
	}
}
