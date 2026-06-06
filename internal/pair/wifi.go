package pair

import (
	"os/exec"
	"time"
)

// qndbWiFi drives ONLY the silent connect path. CLAUDE.md invariant #7:
// NEVER call wfmConnectWireless (non-silent) or pwrReboot — both crash Nickel.
type qndbWiFi struct{}

// NewQndbWiFi builds the on-device WiFi driver.
func NewQndbWiFi() WiFi { return &qndbWiFi{} }

// Connect requests a silent connect and waits up to timeout for the
// wmNetworkConnected signal. `qndb -s wmNetworkConnected` blocks until the
// signal fires; we race it against the timeout.
func (qndbWiFi) Connect(timeout time.Duration) bool {
	// Kick the silent connect (non-blocking on the daemon side).
	_ = exec.Command("qndb", "-m", "wfmConnectWirelessSilently").Run()

	done := make(chan bool, 1)
	cmd := exec.Command("qndb", "-s", "wmNetworkConnected")
	go func() {
		done <- cmd.Run() == nil // exits 0 when the signal fires
	}()

	select {
	case ok := <-done:
		return ok
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill() // stop waiting; radio may be disabled (unrecoverable)
		}
		return false
	}
}
