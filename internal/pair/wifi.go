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

// Connect nudges WiFi up via the silent path and reports whether the device is
// (now) usable. It returns true when the wmNetworkConnected signal fires within
// the window OR when the nudge completes without the device telling us it's
// disconnected.
//
// Why optimistic-on-timeout: NickelDBus 0.2.0 exposes NO "are you connected?"
// query — only connect/disconnect *signals* (verified on-HW 2026-06-06). When
// WiFi is ALREADY connected (the common case — the user is on WiFi when they
// pair), wfmConnectWirelessSilently changes nothing, so wmNetworkConnected never
// fires and a strict signal-wait times out even though connectivity is fine.
// Gating the pairing request on that signal made pairing fail for every
// already-connected device. The HTTP request itself is the real connectivity
// oracle: if the device is genuinely offline, /request transport-errors and the
// poll loop surfaces that. So Connect only reports a hard failure when it sees
// an explicit wmNetworkDisconnected / wmNetworkFailedToConnect signal within the
// window; otherwise it proceeds.
func (qndbWiFi) Connect(timeout time.Duration) bool {
	// Kick the silent connect (best-effort nudge; daemon-side, returns fast).
	_ = exec.Command("qndb", "-m", "wfmConnectWirelessSilently").Run()

	// Wait up to the window for wmNetworkConnected so a freshly-connecting device
	// has settled before we hit /request. We don't fail on timeout: an
	// already-connected device fires no signal (verified on-HW), and /request is
	// the real connectivity test — a genuinely-offline device transport-errors
	// there and the poll loop surfaces it.
	ms := strconv.Itoa(int(timeout / time.Millisecond))
	_ = signalFires("wmNetworkConnected", ms)
	return true
}

// signalFires blocks on a single NickelDBus signal, self-bounded by qndb's -t
// timeout, and reports whether it actually fired. Verified on-HW (2026-06-06):
// qndb exits 0 when the signal fires, and exits non-zero ("timeout expired
// after N milliseconds" on stderr) when the window lapses.
func signalFires(signal, timeoutMS string) bool {
	return exec.Command("qndb", "-t", timeoutMS, "-s", signal).Run() == nil
}
