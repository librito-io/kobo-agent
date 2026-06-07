package autosync

import "time"

// Clock abstracts time for the bounded wait, using MONOTONIC elapsed time, never
// the device wall clock (unreliable on the Kobo). main's realClock satisfies it.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// WaitForConnectivity polls p at cadence until a Ready snapshot or timeout
// elapses (measured on clk). It probes immediately before the first sleep, so an
// already-up link returns without waiting; at a udev `add` none of the signals
// hold yet (~2 s observed to carrier), so the wait closes that gap. Returns true
// iff a Ready snapshot was seen within the bound.
func WaitForConnectivity(p Prober, clk Clock, timeout, cadence time.Duration) bool {
	deadline := clk.Now().Add(timeout)
	for {
		if p.Probe().Ready() {
			return true
		}
		if !clk.Now().Before(deadline) {
			return false
		}
		clk.Sleep(cadence)
	}
}
