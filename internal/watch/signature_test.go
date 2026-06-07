package watch

import "testing"

func TestGrew(t *testing.T) {
	cases := []struct {
		name      string
		prev, cur Signature
		want      bool
	}{
		{"count rose", Signature{7, "2026-06-04T19:00:00.000"}, Signature{8, "2026-06-04T19:00:00.000"}, true},
		{"max date advanced", Signature{7, "2026-06-04T19:00:00.000"}, Signature{7, "2026-06-04T20:00:00.000"}, true},
		{"both flat", Signature{7, "2026-06-04T19:00:00.000"}, Signature{7, "2026-06-04T19:00:00.000"}, false},
		{"delete shrinks count, date flat → not grown", Signature{7, "2026-06-04T19:00:00.000"}, Signature{6, "2026-06-04T19:00:00.000"}, false},
		{"delete-then-add: count flat, date up → grown", Signature{7, "2026-06-04T19:00:00.000"}, Signature{7, "2026-06-04T21:00:00.000"}, true},
		{"both grew", Signature{7, "2026-06-04T19:00:00.000"}, Signature{9, "2026-06-04T22:00:00.000"}, true},
		{"empty baseline → any real count grows", Signature{0, ""}, Signature{1, "2026-06-04T19:00:00.000"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := grew(c.prev, c.cur); got != c.want {
				t.Fatalf("grew(%+v, %+v) = %v, want %v", c.prev, c.cur, got, c.want)
			}
		})
	}
}
