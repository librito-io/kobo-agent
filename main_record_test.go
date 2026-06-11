package main

import "testing"

// defaultRecordPath is the single origin of the record-path derivation shared by
// autosync, status, sync-now, and watch. All four MUST resolve identically — the
// record holds the toast growth baseline (#38), so a drifted filename in one
// handler silently splits the baseline.
func TestDefaultRecordPath(t *testing.T) {
	cases := []struct {
		name, record, dir, want string
	}{
		{"explicit override wins", "/x/rec", "/adds/librito", "/x/rec"},
		{"empty derives from dir", "", "/adds/librito", "/adds/librito/last-sync"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := defaultRecordPath(c.record, c.dir); got != c.want {
				t.Errorf("defaultRecordPath(%q, %q) = %q, want %q", c.record, c.dir, got, c.want)
			}
		})
	}
}
