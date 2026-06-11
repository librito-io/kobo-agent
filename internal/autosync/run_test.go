package autosync

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func deps(l *fakeLocker, cfg fakeConfig, p *fakeProber, s *fakeSyncer, lg *fakeLogger, clk *fakeClock) Deps {
	return Deps{
		Locker: l, Config: cfg, Prober: p, Syncer: s, Logger: lg, Clock: clk,
		Record:     &fakeRecordStore{},
		ViewProber: fakeViewProber{view: "home"},
		Toaster:    &fakeToaster{},
		ToastAllow: []string{"home"},
		Timeout:    60 * time.Second,
		Cadence:    2 * time.Second,
	}
}

func TestRun_HappyPath(t *testing.T) {
	l := &fakeLocker{ok: true}
	cfg := fakeConfig{token: "sk_device_x", baseURL: "http://dev:5173"}
	p := &fakeProber{snaps: []Snapshot{ready()}}
	s := &fakeSyncer{imported: 6, books: 6}
	lg := &fakeLogger{}
	clk := newFakeClock()

	out := Run(deps(l, cfg, p, s, lg, clk))

	if out != OutcomeSynced {
		t.Fatalf("outcome = %d, want OutcomeSynced", out)
	}
	if s.calls != 1 || s.gotURL != "http://dev:5173" || s.gotToken != "sk_device_x" {
		t.Fatalf("syncer call wrong: calls=%d url=%q token=%q", s.calls, s.gotURL, s.gotToken)
	}
	if !l.released {
		t.Fatal("lock not released")
	}
	if len(lg.lines) != 1 || !strings.Contains(lg.lines[0], "autosync: imported 6 across 6 books") {
		t.Fatalf("log lines = %v, want one success line", lg.lines)
	}
}

func TestRun_LockHeld_NoOpExitZero(t *testing.T) {
	l := &fakeLocker{ok: false} // another run holds it
	s := &fakeSyncer{}
	lg := &fakeLogger{}
	out := Run(deps(l, fakeConfig{token: "tok"}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if out != OutcomeDedup {
		t.Fatalf("outcome = %d, want OutcomeDedup", out)
	}
	if s.calls != 0 {
		t.Fatal("must not sync when the lock is held")
	}
	if len(lg.lines) != 0 {
		t.Fatalf("lock-held should log nothing, got %v", lg.lines)
	}
}

func TestRun_LockError_ExitNonzero(t *testing.T) {
	l := &fakeLocker{err: errors.New("flock: permission denied")}
	s := &fakeSyncer{}
	lg := &fakeLogger{}
	out := Run(deps(l, fakeConfig{token: "tok"}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if out != OutcomeLockErr {
		t.Fatalf("outcome = %d, want OutcomeLockErr", out)
	}
	if s.calls != 0 {
		t.Fatal("must not sync on lock error")
	}
	if len(lg.lines) != 1 || !strings.Contains(lg.lines[0], "lock") {
		t.Fatalf("want a logged lock error, got %v", lg.lines)
	}
}

func TestRun_NoToken_ExitZeroNoSync(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSyncer{}
	lg := &fakeLogger{}
	out := Run(deps(l, fakeConfig{token: ""}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if out != OutcomeUnpaired {
		t.Fatalf("outcome = %d, want OutcomeUnpaired", out)
	}
	if s.calls != 0 {
		t.Fatal("unpaired device must not POST")
	}
	if !l.released {
		t.Fatal("lock acquired then must be released on the no-token path")
	}
	if len(lg.lines) != 1 || !strings.Contains(lg.lines[0], "not paired") {
		t.Fatalf("want one 'not paired' line, got %v", lg.lines)
	}
}

func TestRun_NotReadyTimeout_ExitNonzero(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSyncer{}
	lg := &fakeLogger{}
	p := &fakeProber{snaps: []Snapshot{notReady()}} // never ready
	out := Run(deps(l, fakeConfig{token: "tok"}, p, s, lg, newFakeClock()))

	if out != OutcomeOffline {
		t.Fatalf("outcome = %d, want OutcomeOffline", out)
	}
	if s.calls != 0 {
		t.Fatal("must not sync when connectivity never came up")
	}
	if len(lg.lines) != 1 || !strings.Contains(lg.lines[0], "connectivity") {
		t.Fatalf("want a logged connectivity timeout, got %v", lg.lines)
	}
}

func TestRun_SyncError_ExitNonzero(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSyncer{err: errors.New("post import: dial tcp: timeout")}
	lg := &fakeLogger{}
	out := Run(deps(l, fakeConfig{token: "tok", baseURL: "http://dev:5173"}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if out != OutcomeError {
		t.Fatalf("outcome = %d, want OutcomeError", out)
	}
	if len(lg.lines) != 1 || !strings.Contains(lg.lines[0], "post import") {
		t.Fatalf("want the sync error logged, got %v", lg.lines)
	}
}

func TestRun_WaitsThenSyncs(t *testing.T) {
	l := &fakeLocker{ok: true}
	s := &fakeSyncer{imported: 6, books: 6}
	lg := &fakeLogger{}
	p := &fakeProber{snaps: []Snapshot{notReady(), notReady(), ready()}}
	out := Run(deps(l, fakeConfig{token: "tok"}, p, s, lg, newFakeClock()))

	if out != OutcomeSynced {
		t.Fatalf("outcome = %d, want OutcomeSynced", out)
	}
	if p.calls != 3 {
		t.Fatalf("probed %d times, want 3 (waited for the up-edge)", p.calls)
	}
	if s.calls != 1 {
		t.Fatal("want exactly one sync after connectivity came up")
	}
}

func TestRun_HappyPath_RecordsSuccessAndToastsAtHome(t *testing.T) {
	rec := &fakeRecordStore{}
	tst := &fakeToaster{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 1, books: 1},
		&fakeLogger{}, newFakeClock())
	d.Record = rec
	d.Toaster = tst
	// deps() already sets ViewProber to the home view (allow-listed).

	Run(d)

	if len(rec.got) != 1 || rec.got[0] != OutcomeSynced {
		t.Fatalf("record calls = %v, want [Synced]", rec.got)
	}
	if len(tst.mains) != 1 {
		t.Fatalf("want one toast at home, got %v", tst.mains)
	}
}

func TestRun_SuppressesToastWhileReading(t *testing.T) {
	tst := &fakeToaster{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 1, books: 1},
		&fakeLogger{}, newFakeClock())
	d.Toaster = tst
	d.ViewProber = fakeViewProber{view: "reading"}

	Run(d)

	if len(tst.mains) != 0 {
		t.Fatalf("must not toast while reading, got %v", tst.mains)
	}
}

func TestRun_ToastsOnlyWhenSetGrew(t *testing.T) {
	// imported == last recorded count → a plain full-set re-send (invariant #5), not
	// growth → no toast, even though the view is allow-listed (home).
	tst := &fakeToaster{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 5, books: 5},
		&fakeLogger{}, newFakeClock())
	d.Record = &fakeRecordStore{lastCount: 5}
	d.Toaster = tst

	Run(d)

	if len(tst.mains) != 0 {
		t.Fatalf("no growth (5→5) must not toast, got %v", tst.mains)
	}
}

func TestRun_ToastsWhenSetGrewAtHome(t *testing.T) {
	tst := &fakeToaster{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 8, books: 8},
		&fakeLogger{}, newFakeClock())
	d.Record = &fakeRecordStore{lastCount: 5} // 5 → 8 = grew
	d.Toaster = tst

	Run(d)

	if len(tst.mains) != 1 {
		t.Fatalf("growth at home must toast once, got %v", tst.mains)
	}
	if tst.mains[0] != "3 new highlights synced to Librito" {
		t.Fatalf("toast text = %q, want the 5→8 delta spelled out", tst.mains[0])
	}
}

func TestRun_ToastTextSingularForOneNewHighlight(t *testing.T) {
	tst := &fakeToaster{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 6, books: 6},
		&fakeLogger{}, newFakeClock())
	d.Record = &fakeRecordStore{lastCount: 5} // 5 → 6 = one new
	d.Toaster = tst

	Run(d)

	if len(tst.mains) != 1 || tst.mains[0] != "1 new highlight synced to Librito" {
		t.Fatalf("toast = %v, want singular '1 new highlight synced to Librito'", tst.mains)
	}
}

func TestRun_NoViewProbeWhenSetDidNotGrow(t *testing.T) {
	// The grow check gates BEFORE the view probe, so a no-growth wake-reconnect makes
	// ZERO qndb calls (otherwise every watch-path highlight pays a view-probe exec).
	vp := &countingViewProber{view: "home"}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 5, books: 5},
		&fakeLogger{}, newFakeClock())
	d.Record = &fakeRecordStore{lastCount: 5} // no growth
	d.ViewProber = vp
	d.Toaster = &fakeToaster{}

	Run(d)

	if vp.calls != 0 {
		t.Fatalf("must not probe the view when nothing grew, probed %d times", vp.calls)
	}
}

func TestRun_RecordsImportedCount(t *testing.T) {
	rec := &fakeRecordStore{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{imported: 6, books: 6},
		&fakeLogger{}, newFakeClock())
	d.Record = rec

	Run(d)

	if len(rec.counts) != 1 || rec.counts[0] != 6 {
		t.Fatalf("recorded counts = %v, want [6] (the imported total)", rec.counts)
	}
}

func TestRun_NoToastOnFailure(t *testing.T) {
	// Offline + error both return before maybeToast, so a failed sync never toasts
	// regardless of view or growth (acceptance: "Sync failure / offline → no toast").
	cases := []struct {
		name   string
		prober *fakeProber
		syncer *fakeSyncer
	}{
		{"offline", &fakeProber{snaps: []Snapshot{notReady()}}, &fakeSyncer{}},
		{"error", &fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{err: errors.New("post import: timeout")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tst := &fakeToaster{}
			d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t", baseURL: "http://dev"},
				c.prober, c.syncer, &fakeLogger{}, newFakeClock())
			d.Toaster = tst
			Run(d)
			if len(tst.mains) != 0 {
				t.Fatalf("%s must not toast, got %v", c.name, tst.mains)
			}
		})
	}
}

func TestRun_OfflineRecordsOffline_DedupRecordsNothing(t *testing.T) {
	recOff := &fakeRecordStore{}
	d := deps(&fakeLocker{ok: true}, fakeConfig{token: "t"},
		&fakeProber{snaps: []Snapshot{notReady()}}, &fakeSyncer{}, &fakeLogger{}, newFakeClock())
	d.Record = recOff
	if Run(d); len(recOff.got) != 1 || recOff.got[0] != OutcomeOffline {
		t.Fatalf("offline record = %v, want [Offline]", recOff.got)
	}

	recDup := &fakeRecordStore{}
	d2 := deps(&fakeLocker{ok: false}, fakeConfig{token: "t"},
		&fakeProber{snaps: []Snapshot{ready()}}, &fakeSyncer{}, &fakeLogger{}, newFakeClock())
	d2.Record = recDup
	if Run(d2); len(recDup.got) != 0 {
		t.Fatalf("dedup must record nothing, got %v", recDup.got)
	}
}
