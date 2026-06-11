package main

import (
	"path/filepath"
	"testing"
)

// runWatch owns the subcommand's flag surface. --record must parse like its
// siblings (autosync/status/sync-now): the growth-gate baseline lives in the
// last-sync record, so a caller overriding --record on autosync needs the same
// override here or the two runs split the baseline (#38). The nonexistent --db
// directory makes runWatch return 1 before the daemon loop on every platform
// (linux: inotify add fails; non-linux: stub watcher errors), keeping this
// hermetic. Before the flag existed this test died on flag.ExitOnError.
func TestRunWatch_AcceptsRecordFlag(t *testing.T) {
	dir := t.TempDir()

	code := runWatch([]string{
		"--db", filepath.Join(dir, "no-such-dir", "KoboReader.sqlite"),
		"--dir", dir,
		"--record", filepath.Join(dir, "custom-record"),
		"--watch-lock", filepath.Join(dir, "watch.lock"),
		"--lock", filepath.Join(dir, "sync.lock"),
		"--log", filepath.Join(dir, "autosync.log"),
	})
	if code != 1 {
		t.Fatalf("runWatch with a missing watch dir = %d, want 1", code)
	}
}
