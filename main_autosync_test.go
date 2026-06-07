package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runAutosync owns the subcommand's flag defaults + Dep wiring. With no token in
// --dir the no-token guard returns 0 and logs "not paired", exercising dispatch
// → flag parse → Dep wiring → the unpaired path without a device, network, or DB.
// (The connectivity prober and sync are never reached: the token guard precedes
// them, so this stays hermetic on any host.)
func TestRunAutosync_NoTokenExitsZero(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "autosync.log")

	code := runAutosync([]string{
		"--dir", dir,
		"--lock", filepath.Join(dir, "lock"),
		"--log", logPath,
	})
	if code != 0 {
		t.Fatalf("runAutosync with no token = %d, want 0", code)
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(b), "not paired") {
		t.Fatalf("log = %q, want it to mention 'not paired'", string(b))
	}
}
