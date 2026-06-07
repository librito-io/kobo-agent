package autosync

import (
	"testing"
	"time"
)

func ready() Snapshot    { return Snapshot{Carrier: true, OperstateUp: true, DefaultRoute: true} }
func notReady() Snapshot { return Snapshot{} }

func TestWaitForConnectivity_ReadyImmediately(t *testing.T) {
	p := &fakeProber{snaps: []Snapshot{ready()}}
	clk := newFakeClock()
	start := clk.Now()

	if !WaitForConnectivity(p, clk, 60*time.Second, 2*time.Second) {
		t.Fatal("want ready immediately")
	}
	if p.calls != 1 {
		t.Fatalf("probed %d times, want exactly 1 (immediate)", p.calls)
	}
	if !clk.Now().Equal(start) {
		t.Fatal("clock advanced — should not sleep when ready on first probe")
	}
}

func TestWaitForConnectivity_ReadyAfterSomePolls(t *testing.T) {
	p := &fakeProber{snaps: []Snapshot{notReady(), notReady(), ready()}}
	clk := newFakeClock()
	start := clk.Now()

	if !WaitForConnectivity(p, clk, 60*time.Second, 2*time.Second) {
		t.Fatal("want ready after 2 polls")
	}
	if p.calls != 3 {
		t.Fatalf("probed %d times, want 3", p.calls)
	}
	if clk.Now().Sub(start) != 4*time.Second { // two cadence sleeps
		t.Fatalf("elapsed %v, want 4s (2 cadence sleeps)", clk.Now().Sub(start))
	}
}

func TestWaitForConnectivity_TimesOut(t *testing.T) {
	p := &fakeProber{snaps: []Snapshot{notReady()}} // never ready
	clk := newFakeClock()

	if WaitForConnectivity(p, clk, 6*time.Second, 2*time.Second) {
		t.Fatal("want timeout (never ready)")
	}
	// Probes at t=0,2,4,6 → 4 probes; at t=6 the deadline is reached and it stops.
	if p.calls != 4 {
		t.Fatalf("probed %d times, want 4 (t=0,2,4,6)", p.calls)
	}
}
