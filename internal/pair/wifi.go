package pair

import (
	"os/exec"
	"strconv"
	"time"
)

// qndbWiFi drives ONLY the silent connect path. CLAUDE.md invariant #7:
// NEVER call wfmConnectWireless (non-silent) or pwrReboot — both crash Nickel.
type qndbWiFi struct{}

// NewQndbWiFi builds the on-device WiFi driver.
func NewQndbWiFi() WiFi { return &qndbWiFi{} }

// Connect nudges WiFi up via the silent path and then ALWAYS proceeds — it
// always returns true.
//
// Why always-optimistic: NickelDBus 0.2.0 exposes NO "are you connected?" query
// — only connect/disconnect *signals* (verified on-HW 2026-06-06). When WiFi is
// ALREADY connected (the common case — the user is on WiFi when they pair),
// wfmConnectWirelessSilently changes nothing, so wmNetworkConnected never fires
// and a strict signal-wait times out even though connectivity is fine. Gating
// the pairing request on that signal made pairing fail for every already-
// connected device. So Connect waits the window for wmNetworkConnected only to
// let a freshly-connecting device settle, then proceeds regardless: the HTTP
// /request is the real connectivity oracle — a genuinely-offline device
// transport-errors there and the poll loop surfaces it. (Routing that transport
// failure to a No-WiFi dialog is tracked as a followup.)
func (qndbWiFi) Connect(timeout time.Duration) bool {
	// Kick the silent connect (best-effort nudge; daemon-side, returns fast).
	_ = exec.Command("qndb", "-m", "wfmConnectWirelessSilently").Run()

	// Wait up to the window for wmNetworkConnected so a freshly-connecting device
	// has settled before we hit /request. We don't fail on timeout: an
	// already-connected device fires no signal (verified on-HW), and /request is
	// the real connectivity test — a genuinely-offline device transport-errors
	// there and the poll loop surfaces it.
	_ = signalFires("wmNetworkConnected", millis(timeout))
	return true
}

// millis renders a duration as the integer-millisecond string qndb's -t flag
// expects (clamped at 0 so a negative timeout can't become a huge unsigned wait).
func millis(d time.Duration) string {
	ms := d / time.Millisecond
	if ms < 0 {
		ms = 0
	}
	return strconv.Itoa(int(ms))
}

// signalFires blocks on a single NickelDBus signal, self-bounded by qndb's -t
// timeout, and reports whether it actually fired. Verified on-HW (2026-06-06):
// qndb exits 0 when the signal fires, and exits non-zero ("timeout expired
// after N milliseconds" on stderr) when the window lapses.
func signalFires(signal, timeoutMS string) bool {
	return exec.Command("qndb", "-t", timeoutMS, "-s", signal).Run() == nil
}
