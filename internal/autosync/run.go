package autosync

import (
	"fmt"
	"time"
)

// Deps wires Run to its impure edges (fakes in tests, real impls in main).
type Deps struct {
	Locker Locker
	Config Config
	Prober Prober
	Syncer Syncer
	Logger Logger
	Clock  Clock

	Timeout time.Duration // connectivity-wait bound (60s)
	Cadence time.Duration // poll cadence (~2s)
}

// Run executes one autosync trigger and returns a process exit code:
//
//	lock → no-token guard → wait-connectivity → resolve-url → sync → log
//
// Exit 0 on success, dedup (lock held), or unpaired no-op; nonzero on a lock
// error, connectivity timeout, or sync failure (each logged). The sync is
// idempotent + additive server-side, so retry-on-next-up-edge is safe. This path
// performs NO WiFi control — it only reacts to a WiFi-up event (invariant #7).
func Run(d Deps) int {
	// 1. Lock — non-blocking. Held by another run → quiet dedup, exit 0.
	ok, unlock, err := d.Locker.TryLock()
	if err != nil {
		d.log("lock error: " + err.Error())
		return 1
	}
	if !ok {
		return 0 // another run is active; it handles this WiFi-up
	}
	defer unlock()

	// 2. No-token guard — an unpaired device must not POST an empty bearer (and
	// pairing's own silent connect can fire this trigger mid-pair).
	token := d.Config.Token()
	if token == "" {
		d.log("skipped: not paired")
		return 0
	}

	// 3. Wait for connectivity — bounded; a timeout retries on the next up-edge.
	if !WaitForConnectivity(d.Prober, d.Clock, d.Timeout, d.Cadence) {
		d.log(fmt.Sprintf("no connectivity within %s", d.Timeout))
		return 1
	}

	// 4. Resolve the paired backend + 5. sync (the HTTP POST is the final
	// connectivity oracle — a device that looked ready but isn't errors here).
	imported, books, err := d.Syncer.Sync(d.Config.BaseURL(), token)
	if err != nil {
		d.log(err.Error())
		return 1
	}

	// 6. Log success.
	d.log(fmt.Sprintf("imported %d across %d books", imported, books))
	return 0
}

// log composes a UTC line (Clock.Now is normalized to UTC by FormatLine) and
// appends it via the Logger. Best-effort — a log failure never changes the exit.
func (d Deps) log(msg string) {
	d.Logger.Log(FormatLine(d.Clock.Now(), msg))
}
