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

// qndbBin is the absolute path to NickelDBus's qndb CLI. It MUST be absolute:
// the WiFi-up autosync is launched by udev, and udev spawns RUN children with NO
// PATH in their environment. A bare "qndb" would fail exec.LookPath there, so the
// view probe and the toast would silently no-op — the (real) reason the Step-4
// post-sync toast never fired on the wake/udev path, even though the PATH-free
// Go-HTTP sync still ran. /usr/bin/qndb is NDB's fixed on-device install location.
const qndbBin = "/usr/bin/qndb"

// qndbViewProber calls `qndb -m ndbCurrentView` (a GETTER in NDB 0.2.0) and returns
// its trimmed stdout. Any failure → "" (ShouldToast then fails safe → no toast).
type qndbViewProber struct{}

// NewQndbViewProber builds the on-device ViewProber.
func NewQndbViewProber() ViewProber { return qndbViewProber{} }

func (qndbViewProber) CurrentView() string {
	out, err := exec.Command(qndbBin, "-m", "ndbCurrentView").Output()
	if err != nil {
		return ""
	}
	return lastLineField(string(out))
}

// qndbToaster fires `qndb -m mwcToast <durationMs> <main> <sub>`. Errors swallowed.
type qndbToaster struct{ durationMs string }

// NewQndbToaster builds a Toaster with a fixed duration (ms). TUNE/clamp per the
// hardware-owed toast-duration ceiling.
func NewQndbToaster(durationMs string) Toaster { return qndbToaster{durationMs: durationMs} }

func (t qndbToaster) Toast(main, sub string) {
	_ = exec.Command(qndbBin, "-m", "mwcToast", t.durationMs, main, sub).Run()
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
