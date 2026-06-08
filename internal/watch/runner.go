package watch

import (
	"fmt"

	"github.com/librito-io/kobo-agent/internal/autosync"
)

// autosyncRunner delegates the whole sync to autosync.Run (lock → token-guard →
// connectivity-wait → resolve-url → sync.Run → log), reusing the shared
// /tmp/librito-autosync.lock so the daemon and a udev-fired run never double-run.
type autosyncRunner struct{ deps autosync.Deps }

// NewRunner builds a Runner over a configured autosync.Deps. The caller sets a
// SHORT Timeout (we only call Sync when already connected, so a long wait is
// never wanted; a rare WiFi-drops-in-the-race then fails fast).
func NewRunner(deps autosync.Deps) Runner { return &autosyncRunner{deps: deps} }

// Sync maps autosync.Run's Outcome.ExitCode() to error: 0 (success / dedup / unpaired-
// noop) → nil; nonzero → error. The caller advances its baseline on nil — correct
// even for the exit-0 dedup/unpaired cases (a concurrent run is resending the full
// set, or there is nothing to sync).
func (r *autosyncRunner) Sync() error {
	if code := autosync.Run(r.deps).ExitCode(); code != 0 {
		return fmt.Errorf("autosync exit %d", code)
	}
	return nil
}
