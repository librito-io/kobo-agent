package autosync

import (
	"os/exec"
	"strings"
)

// ViewProber reports Nickel's current view (for the toast gate). Best-effort.
type ViewProber interface {
	CurrentView() string
}

// Toaster shows a Nickel toast. Best-effort — a dead NDB loses the toast, not the
// sync (spec: NDB garnish on the mod-independent backbone).
type Toaster interface {
	Toast(main, sub string)
}

// qndbViewProber calls `qndb -m ndbCurrentView` (a GETTER in NDB 0.2.0) and returns
// its trimmed stdout. Any failure → "" (ShouldToast then fails safe → no toast).
type qndbViewProber struct{}

// NewQndbViewProber builds the on-device ViewProber.
func NewQndbViewProber() ViewProber { return qndbViewProber{} }

func (qndbViewProber) CurrentView() string {
	out, err := exec.Command("qndb", "-m", "ndbCurrentView").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(lastLineField(string(out)))
}

// qndbToaster fires `qndb -m mwcToast <durationMs> <main> <sub>`. Errors swallowed.
type qndbToaster struct{ durationMs string }

// NewQndbToaster builds a Toaster with a fixed duration (ms). TUNE/clamp per the
// hardware-owed toast-duration ceiling.
func NewQndbToaster(durationMs string) Toaster { return qndbToaster{durationMs: durationMs} }

func (t qndbToaster) Toast(main, sub string) {
	_ = exec.Command("qndb", "-m", "mwcToast", t.durationMs, main, sub).Run()
}

// noopViewProber / noopToaster disable the in-Run toast (sync-now uses these: it
// shows its own "Syncing…" toast + a result dialog instead).
type noopViewProber struct{}

func (noopViewProber) CurrentView() string { return "" }

type noopToaster struct{}

func (noopToaster) Toast(string, string) {}

// NewNoopViewProber / NewNoopToaster expose the no-ops for sync-now wiring.
func NewNoopViewProber() ViewProber { return noopViewProber{} }
func NewNoopToaster() Toaster       { return noopToaster{} }

// lastLineField returns the last whitespace token of the last non-empty line of s
// (qndb may print a signal/name prefix before the value, mirroring pair.lastField).
func lastLineField(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	last := lines[len(lines)-1]
	f := strings.Fields(last)
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}
