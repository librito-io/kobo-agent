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
		Timeout: 60 * time.Second,
		Cadence: 2 * time.Second,
	}
}

func TestRun_HappyPath(t *testing.T) {
	l := &fakeLocker{ok: true}
	cfg := fakeConfig{token: "sk_device_x", baseURL: "http://dev:5173"}
	p := &fakeProber{snaps: []Snapshot{ready()}}
	s := &fakeSyncer{imported: 6, books: 6}
	lg := &fakeLogger{}
	clk := newFakeClock()

	code := Run(deps(l, cfg, p, s, lg, clk))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
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
	code := Run(deps(l, fakeConfig{token: "tok"}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (quiet dedup)", code)
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
	code := Run(deps(l, fakeConfig{token: "tok"}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if code == 0 {
		t.Fatal("exit code = 0, want nonzero on lock error")
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
	code := Run(deps(l, fakeConfig{token: ""}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (unpaired no-op)", code)
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
	code := Run(deps(l, fakeConfig{token: "tok"}, p, s, lg, newFakeClock()))

	if code == 0 {
		t.Fatal("exit code = 0, want nonzero on connectivity timeout")
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
	code := Run(deps(l, fakeConfig{token: "tok", baseURL: "http://dev:5173"}, &fakeProber{snaps: []Snapshot{ready()}}, s, lg, newFakeClock()))

	if code == 0 {
		t.Fatal("exit code = 0, want nonzero on sync error")
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
	code := Run(deps(l, fakeConfig{token: "tok"}, p, s, lg, newFakeClock()))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if p.calls != 3 {
		t.Fatalf("probed %d times, want 3 (waited for the up-edge)", p.calls)
	}
	if s.calls != 1 {
		t.Fatal("want exactly one sync after connectivity came up")
	}
}
