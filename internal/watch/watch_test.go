package watch

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRun_LockHeld_NoOpExitZero(t *testing.T) {
	l := &fakeLocker{ok: false}
	r := &fakeRunner{}
	lg := &fakeLogger{}
	code := Run(deps(l, newFakeWatcher(), &fakeSigReader{sigs: []Signature{{0, ""}}}, &fakeProber{true}, r, lg))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (another daemon owns the lock)", code)
	}
	if r.calls != 0 {
		t.Fatal("must not sync when the lock is held")
	}
	if len(lg.lines) != 0 {
		t.Fatalf("lock-held should log nothing, got %v", lg.lines)
	}
}

func TestRun_LockError_ExitNonzero(t *testing.T) {
	l := &fakeLocker{err: errors.New("flock: permission denied")}
	r := &fakeRunner{}
	lg := &fakeLogger{}
	code := Run(deps(l, newFakeWatcher(), &fakeSigReader{sigs: []Signature{{0, ""}}}, &fakeProber{true}, r, lg))
	if code == 0 {
		t.Fatal("exit = 0, want nonzero on lock error")
	}
	if r.calls != 0 {
		t.Fatal("must not sync on lock error")
	}
}

func TestRun_GrowsAndReady_Syncs(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSigReader{sigs: []Signature{{7, "2026-06-04T19:00:00.000"}, {8, "2026-06-04T20:00:00.000"}}}
	r := &fakeRunner{}
	lg := &fakeLogger{}
	code := Run(deps(l, newFakeWatcher(walEvent()), s, &fakeProber{true}, r, lg))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if r.calls != 1 {
		t.Fatalf("sync calls = %d, want 1", r.calls)
	}
	if !l.released {
		t.Fatal("lock not released")
	}
	if !containsLine(lg.lines, "watch: signature grew 7→8") {
		t.Fatalf("missing grew line, got %v", lg.lines)
	}
}

func TestRun_GrowsNotReady_NoSync(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSigReader{sigs: []Signature{{7, "2026-06-04T19:00:00.000"}, {8, "2026-06-04T20:00:00.000"}}}
	r := &fakeRunner{}
	lg := &fakeLogger{}
	Run(deps(l, newFakeWatcher(walEvent()), s, &fakeProber{false}, r, lg))
	if r.calls != 0 {
		t.Fatalf("sync calls = %d, want 0 (offline)", r.calls)
	}
	if !containsLine(lg.lines, "watch: skipped: not connected") {
		t.Fatalf("missing skip line, got %v", lg.lines)
	}
}

func TestRun_NoGrowth_NoSync(t *testing.T) {
	l := &fakeLocker{ok: true}
	base := Signature{7, "2026-06-04T19:00:00.000"}
	s := &fakeSigReader{sigs: []Signature{base, base}} // baseline then same → no growth
	r := &fakeRunner{}
	lg := &fakeLogger{}
	Run(deps(l, newFakeWatcher(walEvent()), s, &fakeProber{true}, r, lg))
	if r.calls != 0 {
		t.Fatalf("sync calls = %d, want 0 (no growth)", r.calls)
	}
}

func TestRun_CoalescesBurst(t *testing.T) {
	// Three buffered WAL events with a large debounce window: drain consumes the
	// whole burst (each event hits the event case; the closed channel — always
	// ready to receive — then ends drain), so the loop evaluates ONCE. The
	// discriminator is SigReader.calls: coalesced = baseline + 1 evaluate = 2;
	// uncoalesced (Window=0) would be baseline + 3 = 4. No real timer fires
	// (NewTimer(window) is created and Stop()'d each iteration; window never elapses).
	l := &fakeLocker{ok: true}
	s := &fakeSigReader{sigs: []Signature{{7, "d0"}, {8, "d1"}}}
	r := &fakeRunner{}
	lg := &fakeLogger{}
	d := deps(l, newFakeWatcher(walEvent(), walEvent(), walEvent()), s, &fakeProber{true}, r, lg)
	d.Window = time.Hour // large → the burst is absorbed before any timeout
	code := Run(d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if s.calls != 2 {
		t.Fatalf("SigReader calls = %d, want 2 (baseline + ONE evaluate for the 3-event burst)", s.calls)
	}
	if r.calls != 1 {
		t.Fatalf("sync calls = %d, want 1 (the burst coalesced into one sync)", r.calls)
	}
}

func TestRun_NonWalEvent_Ignored(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSigReader{sigs: []Signature{{7, "2026-06-04T19:00:00.000"}}}
	r := &fakeRunner{}
	lg := &fakeLogger{}
	// Only a -shm event: baseline read happens, but no evaluate (so SigReader is
	// read exactly once, and nothing syncs).
	Run(deps(l, newFakeWatcher(Event{Name: "KoboReader.sqlite-shm", Mask: 0x2}), s, &fakeProber{true}, r, lg))
	if r.calls != 0 {
		t.Fatal("non-WAL event must not trigger a sync")
	}
	if s.calls != 1 {
		t.Fatalf("SigReader calls = %d, want 1 (baseline only, no evaluate)", s.calls)
	}
}

func TestRun_SyncError_HoldsBaseline(t *testing.T) {
	l := &fakeLocker{ok: true}
	// baseline {7}, then two growth reads {8},{8}. First event: grew→sync errors→
	// hold prev at {7}. Second event: cur {8} still > held {7} → sync attempted again.
	s := &fakeSigReader{sigs: []Signature{{7, "d0"}, {8, "d1"}, {8, "d1"}}}
	r := &fakeRunner{err: errors.New("post import: dial tcp: timeout")}
	lg := &fakeLogger{}
	Run(deps(l, newFakeWatcher(walEvent(), walEvent()), s, &fakeProber{true}, r, lg))
	if r.calls != 2 {
		t.Fatalf("sync calls = %d, want 2 (baseline held after error → retried)", r.calls)
	}
	if !containsLine(lg.lines, "watch: sync error: post import") {
		t.Fatalf("missing logged sync error, got %v", lg.lines)
	}
}

func containsLine(lines []string, substr string) bool {
	for _, l := range lines {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}
