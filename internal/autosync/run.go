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

	// Step-4 additions. All optional (nil-safe) so existing callers/tests that
	// don't set them keep working; sync-now sets no-op View/Toaster on purpose.
	Record     RecordStore // last-sync record; nil → not written
	ViewProber ViewProber  // current Nickel view for the toast gate; nil → no toast
	Toaster    Toaster     // post-sync toast; nil → no toast
	ToastAllow []string    // allowlist of views to toast on; nil → defaultToastAllow

	Timeout time.Duration // connectivity-wait bound
	Cadence time.Duration // poll cadence
}

// Run executes one sync trigger and returns a classified Outcome (use
// Outcome.ExitCode() for a process exit code):
//
//	lock → no-token guard → wait-connectivity → resolve-url → sync → record → toast
//
// The sync is idempotent + additive server-side, so retry-on-next-up-edge is safe.
// This path performs NO WiFi control — it only reacts to connectivity (invariant #7).
func Run(d Deps) Outcome {
	ok, unlock, err := d.Locker.TryLock()
	if err != nil {
		d.log("lock error: " + err.Error())
		return OutcomeLockErr
	}
	if !ok {
		return OutcomeDedup // another run is active; it owns this trigger + the record
	}
	defer unlock()

	token := d.Config.Token()
	if token == "" {
		d.log("skipped: not paired")
		return OutcomeUnpaired
	}

	if !WaitForConnectivity(d.Prober, d.Clock, d.Timeout, d.Cadence) {
		d.log(fmt.Sprintf("no connectivity within %s", d.Timeout))
		d.record(OutcomeOffline)
		return OutcomeOffline
	}

	imported, books, err := d.Syncer.Sync(d.Config.BaseURL(), token)
	if err != nil {
		d.log(err.Error())
		d.record(OutcomeError)
		return OutcomeError
	}

	d.log(fmt.Sprintf("imported %d across %d books", imported, books))
	d.record(OutcomeSynced)
	d.maybeToast()
	return OutcomeSynced
}

func (d Deps) log(msg string) {
	d.Logger.Log(FormatLine(d.Clock.Now(), "autosync", msg))
}

func (d Deps) record(o Outcome) {
	if d.Record != nil {
		d.Record.Record(o)
	}
}

// maybeToast fires the best-effort post-sync toast iff a Toaster + ViewProber are
// wired and the current view is allow-listed (never over a book). sync-now leaves
// these nil → no in-Run toast (it owns its own feedback).
func (d Deps) maybeToast() {
	if d.Toaster == nil || d.ViewProber == nil {
		return
	}
	allow := d.ToastAllow
	if allow == nil {
		allow = defaultToastAllow
	}
	if ShouldToast(d.ViewProber.CurrentView(), allow) {
		d.Toaster.Toast("Highlights synced", "")
	}
}
