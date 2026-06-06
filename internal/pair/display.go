package pair

import (
	"os/exec"
	"strings"
	"sync"
	"time"
)

// settleDelay spaces consecutive qndb dialog calls. The Step-2 probe found a
// back-to-back create→show→update burst raced and rendered nothing; ~150ms is
// the starting point — TUNE ON HARDWARE (Task 11), pick the smallest reliable.
const settleDelay = 150 * time.Millisecond

// qndbDisplay drives the single live-updating NickelDBus dialog via `qndb`.
// All methods are best-effort: a failed qndb call logs nothing and returns —
// the pairing loop must not depend on display succeeding.
type qndbDisplay struct {
	mu       sync.Mutex
	lastBtn  Button // latched from the dlgConfirmResult watcher
	watching bool
}

// NewQndbDisplay builds the on-device Display.
func NewQndbDisplay() Display { return &qndbDisplay{} }

func (d *qndbDisplay) ShowCode(code string) {
	// dlgConfirmAcceptReject gives a detectable Cancel button (reject=0).
	// Code goes in the title (renders larger than the body on Nickel).
	d.qndb("dlgConfirmAcceptReject", "Librito pairing", "Enter code "+spaceOut(code)+" at librito.io", "OK", "Cancel")
	d.startResultWatcher()
}

func (d *qndbDisplay) UpdatePaired(email string) {
	d.settle()
	body := "Paired ✓"
	if email != "" {
		body += "  (" + email + ")"
	}
	d.qndb("dlgConfirmSetBody", body)
}

func (d *qndbDisplay) UpdateExpired() {
	d.settle()
	// Re-show with a Retry button (accept=1).
	d.qndb("dlgConfirmAcceptReject", "Librito pairing", "Code expired", "Retry", "Cancel")
	d.startResultWatcher()
}

func (d *qndbDisplay) UpdateNoWiFi() {
	d.settle()
	d.qndb("dlgConfirmAcceptReject", "Librito pairing", "No WiFi", "Retry", "Cancel")
	d.startResultWatcher()
}

func (d *qndbDisplay) UpdateError(msg string) {
	d.settle()
	d.qndb("dlgConfirmSetBody", msg)
}

func (d *qndbDisplay) Close() {
	d.settle()
	d.qndb("dlgConfirmClose")
}

// Poll returns the latest button tap (non-blocking), then clears it.
func (d *qndbDisplay) Poll() Button {
	d.mu.Lock()
	defer d.mu.Unlock()
	b := d.lastBtn
	d.lastBtn = ButtonNone
	return b
}

// startResultWatcher launches a one-shot `qndb -s dlgConfirmResult` that blocks
// until the user taps a button, then latches it. reject=0 → Cancel, accept=1 →
// Retry (the only accept button we show post-code is Retry; the code dialog's
// "OK" accept is harmless — the poll loop's cancel check ignores ButtonRetry).
func (d *qndbDisplay) startResultWatcher() {
	d.mu.Lock()
	if d.watching {
		d.mu.Unlock()
		return
	}
	d.watching = true
	d.mu.Unlock()

	go func() {
		out, err := exec.Command("qndb", "-s", "dlgConfirmResult").Output()
		d.mu.Lock()
		d.watching = false
		if err == nil {
			switch strings.TrimSpace(lastField(string(out))) {
			case "0":
				d.lastBtn = ButtonCancel
			case "1":
				d.lastBtn = ButtonRetry
			}
		}
		d.mu.Unlock()
	}()
}

func (d *qndbDisplay) qndb(method string, args ...string) {
	_ = exec.Command("qndb", append([]string{"-m", method}, args...)...).Run()
}

func (d *qndbDisplay) settle() { time.Sleep(settleDelay) }

// spaceOut renders "482913" as "4 8 2 9 1 3" for legibility.
func spaceOut(code string) string {
	return strings.Join(strings.Split(code, ""), " ")
}

// lastField returns the last whitespace-delimited token of s (qndb prints the
// signal name then the int value).
func lastField(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}
