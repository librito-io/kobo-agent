package autosync

// Outcome classifies one Run. It is Run's return value (replaces the old bare
// int) so callers — especially sync-now — can react without re-reading the
// record file. Distinct from sync.Outcome (a different package's type).
type Outcome int

const (
	OutcomeSynced   Outcome = iota // synced ok           → record success+attempt, exit 0
	OutcomeOffline                 // connectivity timeout → record attempt(offline), exit 1
	OutcomeError                   // sync POST error     → record attempt(error),   exit 1
	OutcomeDedup                   // lock held by another → NO record,              exit 0
	OutcomeUnpaired                // no token            → NO record,               exit 0
	OutcomeLockErr                 // lock system error   → NO record,               exit 1
)

// ExitCode maps an Outcome to the process exit code, preserving the codes the old
// int-returning Run produced (success/dedup/unpaired 0; offline/error/lockerr 1).
func (o Outcome) ExitCode() int {
	switch o {
	case OutcomeOffline, OutcomeError, OutcomeLockErr:
		return 1
	default:
		return 0
	}
}

// defaultToastAllow is the allowlist of ndbCurrentView() strings on which the
// post-sync toast may fire. ALLOWLIST (not denylist): an unknown/new view → no
// toast, so the gate fails SAFE (never a banner over a book). Start with home only.
//
// ⚠ HARDWARE-OWED (spec Owed #2): "home" is a best guess — replace with the exact
// ndbCurrentView() return string for the home screen once verified on-device. A
// wrong value only suppresses toasts; it can never fire one over a book.
var defaultToastAllow = []string{"home"}

// ShouldToast reports whether a post-sync toast may fire on the given view.
func ShouldToast(view string, allow []string) bool {
	for _, v := range allow {
		if view == v {
			return true
		}
	}
	return false
}
