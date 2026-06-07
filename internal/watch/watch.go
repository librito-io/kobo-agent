package watch

import (
	"fmt"
	"time"

	"github.com/librito-io/kobo-agent/internal/autosync"
)

// SigReader reads the current highlight signature (impl wraps
// kobo.ReadHighlightSignature; tests use a fake).
type SigReader interface {
	Read() (Signature, error)
}

// Runner triggers the delegated sync. The autosync.Run adapter is in runner.go;
// tests use a fake. nil = exit 0 (success / dedup / unpaired-noop); error = a
// nonzero autosync exit.
type Runner interface {
	Sync() error
}

// Reused autosync edges (same interfaces, no duplication).
type (
	Prober = autosync.Prober
	Locker = autosync.Locker
	Clock  = autosync.Clock
	Logger = autosync.Logger
)

// Deps wires Run to its edges (fakes in tests, real impls in main).
type Deps struct {
	Locker    Locker        // single-instance /tmp/librito-watch.lock
	Watcher   Watcher       // inotify on the .kobo dir
	SigReader SigReader     // lightweight signature read
	Prober    Prober        // connectivity gate (reused sysfs prober)
	Runner    Runner        // delegated autosync.Run
	Logger    Logger        // shared autosync.log
	Clock     Clock         // monotonic now for debounce
	WALName   string        // basename we react to, e.g. "KoboReader.sqlite-wal"
	Window    time.Duration // debounce silence window (prod 5s; tests 0)
	Cap       time.Duration // debounce max-wait cap (prod 15s)
}

// Run is the resident daemon: single-instance lock → baseline → inotify loop.
// It returns a process exit code. 0 when another daemon already holds the lock
// (idempotent relaunch) or when the watcher channel closes (shutdown); 1 on a
// lock error. It performs NO WiFi control (invariant #7) — only Prober reads.
func Run(d Deps) int {
	ok, unlock, err := d.Locker.TryLock()
	if err != nil {
		d.log("lock error: " + err.Error())
		return 1
	}
	if !ok {
		return 0 // another watch daemon is alive; this relaunch is a no-op
	}
	defer unlock()

	d.log("started")

	// Baseline: read the current signature without syncing. A read error degrades
	// to a zero baseline (the first real growth then syncs).
	prev, err := d.SigReader.Read()
	if err != nil {
		d.log("baseline read error: " + err.Error())
		prev = Signature{}
	}

	events := d.Watcher.Events()
	for {
		ev, ok := <-events
		if !ok {
			return 0 // watcher closed → shutdown
		}
		if ev.Name != d.WALName {
			continue // ignore non-WAL churn (shm, the main db, other files)
		}
		d.drain(events)
		prev = d.evaluate(prev)
	}
}

// drain coalesces a burst: it waits until Window of silence after the last WAL
// event, or Cap after the first, whichever comes first. Non-WAL events seen mid-
// drain do not extend the window. With Window == 0 (tests) it returns immediately.
func (d Deps) drain(events <-chan Event) {
	first := d.Clock.Now()
	last := first
	for {
		wait := debounceWait(first, last, d.Clock.Now(), d.Window, d.Cap)
		if wait <= 0 {
			return
		}
		timer := time.NewTimer(wait)
		select {
		case ev, ok := <-events:
			timer.Stop()
			if !ok {
				return // channel closed; outer loop will detect it
			}
			if ev.Name == d.WALName {
				last = d.Clock.Now()
			}
		case <-timer.C:
			return
		}
	}
}

// evaluate reads the current signature, decides, and acts. It runs the body under
// recover() so a panic is logged-and-continued, NOT a daemon death (per-iteration,
// not loop-scope). Returns the new baseline.
func (d Deps) evaluate(prev Signature) (next Signature) {
	next = prev
	defer func() {
		if r := recover(); r != nil {
			d.log(fmt.Sprintf("recovered panic: %v", r))
		}
	}()

	cur, err := d.SigReader.Read()
	if err != nil {
		d.log("signature read error: " + err.Error())
		return prev
	}

	switch decide(prev, cur, d.Prober.Probe().Ready()) {
	case actionSync:
		if err := d.Runner.Sync(); err != nil {
			d.log("sync error: " + err.Error())
			return prev // hold baseline → retry on the next event
		}
		// Sync() returns nil on exit-0, which includes dedup (a concurrent udev run
		// held the shared lock) and unpaired-noop — neither is a confirmed import.
		// So word this as "delegated", not "imported"; the following autosync.log
		// line ("autosync: imported …" or "autosync: skipped: not paired") is the
		// real outcome.
		d.log(fmt.Sprintf("signature grew %d→%d (delegated to autosync)", prev.Count, cur.Count))
		return cur
	case actionSkipNotReady:
		d.log("skipped: not connected")
		return prev // hold baseline → retry on the next event / udev up-edge
	default: // actionSkipNoGrowth
		return cur // absorb chatty no-op writes into the baseline
	}
}

func (d Deps) log(msg string) {
	d.Logger.Log(autosync.FormatLine(d.Clock.Now(), "watch", msg))
}
