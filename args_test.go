package main

import (
	"path/filepath"
	"testing"
)

func TestPositionalsErr(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no leftovers", nil, ""},
		{"empty slice", []string{}, ""},
		{"single positional", []string{"junk"}, `error: unexpected argument "junk"`},
		{"positional swallows flags", []string{"junk", "--dry-run"},
			`error: unexpected argument "junk" (the 1 arg after it was not parsed)`},
		{"several swallowed", []string{"junk", "--url", "x"},
			`error: unexpected argument "junk" (the 2 args after it were not parsed)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := positionalsErr(tt.args); got != tt.want {
				t.Errorf("positionalsErr(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// Each runner takes flags only — a trailing positional must exit 2 instead of
// silently dropping the flags Parse never reached (#33). Every case is
// hermetic: the positional check fires before any device/network/DB work.
func TestRunners_RejectTrailingPositionals(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "lock")
	log := filepath.Join(dir, "log")

	tests := []struct {
		name string
		run  func([]string) int
		args []string
	}{
		{"sync", runSync, []string{"--dry-run", "junk"}},
		{"status", runStatus, []string{"--dir", dir, "junk"}},
		{"about", runAbout, []string{"--dir", dir, "junk"}},
		{"autosync", runAutosync, []string{"--dir", dir, "--lock", lock, "--log", log, "junk"}},
		{"sync-now", runSyncNow, []string{"--dir", dir, "--lock", lock, "--log", log, "junk"}},
		{"watch", runWatch, []string{"--dir", dir, "junk"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := tt.run(tt.args); code != 2 {
				t.Errorf("%s with trailing positional = %d, want 2", tt.name, code)
			}
		})
	}
}
