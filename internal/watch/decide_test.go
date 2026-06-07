package watch

import "testing"

func TestDecide(t *testing.T) {
	base := Signature{7, "2026-06-04T19:00:00.000"}
	grown := Signature{8, "2026-06-04T20:00:00.000"}

	cases := []struct {
		name      string
		prev, cur Signature
		ready     bool
		want      action
	}{
		{"no growth → skip-no-growth (ready)", base, base, true, actionSkipNoGrowth},
		{"no growth → skip-no-growth (not ready)", base, base, false, actionSkipNoGrowth},
		{"grew + ready → sync", base, grown, true, actionSync},
		{"grew + not ready → skip-not-ready", base, grown, false, actionSkipNotReady},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decide(c.prev, c.cur, c.ready); got != c.want {
				t.Fatalf("decide = %v, want %v", got, c.want)
			}
		})
	}
}
