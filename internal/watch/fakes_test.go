package watch

import (
	"time"

	"github.com/librito-io/kobo-sync/internal/autosync"
)

// fakeClock: fixed monotonic time (debounce uses Now; Window==0 in tests means
// Now never needs to advance).
type fakeClock struct{ t time.Time }

func newFakeClock() *fakeClock             { return &fakeClock{t: time.Date(2026, 6, 7, 19, 42, 0, 0, time.UTC)} }
func (c *fakeClock) Now() time.Time        { return c.t }
func (c *fakeClock) Sleep(d time.Duration) { c.t = c.t.Add(d) }

// fakeLocker: scripted lock outcome; records release.
type fakeLocker struct {
	ok       bool
	err      error
	released bool
}

func (l *fakeLocker) TryLock() (bool, func(), error) {
	if l.err != nil {
		return false, func() {}, l.err
	}
	if !l.ok {
		return false, func() {}, nil
	}
	return true, func() { l.released = true }, nil
}

// fakeWatcher: a buffered channel the test fills then closes.
type fakeWatcher struct{ ch chan Event }

func newFakeWatcher(evs ...Event) *fakeWatcher {
	ch := make(chan Event, len(evs))
	for _, e := range evs {
		ch <- e
	}
	close(ch) // closed channel → Run's loop drains buffered events then shuts down
	return &fakeWatcher{ch: ch}
}
func (w *fakeWatcher) Events() <-chan Event { return w.ch }
func (w *fakeWatcher) Close() error         { return nil }

// fakeSigReader: scripted sequence of signatures; the last repeats. The first
// Read() is the baseline. If err is set, it is returned on call number errOnCall
// (1-based); errOnCall == 0 means err applies to every call.
type fakeSigReader struct {
	sigs      []Signature
	err       error
	errOnCall int
	calls     int
}

func (s *fakeSigReader) Read() (Signature, error) {
	s.calls++
	if s.err != nil && (s.errOnCall == 0 || s.errOnCall == s.calls) {
		return Signature{}, s.err
	}
	i := s.calls - 1
	if i >= len(s.sigs) {
		i = len(s.sigs) - 1
	}
	return s.sigs[i], nil
}

// fakeProber: returns a fixed snapshot (ready/not).
type fakeProber struct{ ready bool }

func (p *fakeProber) Probe() autosync.Snapshot {
	return autosync.Snapshot{Carrier: p.ready, OperstateUp: p.ready, DefaultRoute: p.ready}
}

// fakeRunner: records calls; scripted error.
type fakeRunner struct {
	err   error
	calls int
}

func (r *fakeRunner) Sync() error { r.calls++; return r.err }

// fakeLogger: records every line.
type fakeLogger struct{ lines []string }

func (l *fakeLogger) Log(line string) { l.lines = append(l.lines, line) }

// walName is the event name the daemon reacts to in tests.
const walName = "KoboReader.sqlite-wal"

func walEvent() Event { return Event{Name: walName, Mask: 0x2} }

// deps assembles a Deps with Window==0 (no real debounce timer) for fast tests.
func deps(l *fakeLocker, w Watcher, s *fakeSigReader, p *fakeProber, r *fakeRunner, lg *fakeLogger) Deps {
	return Deps{
		Locker: l, Watcher: w, SigReader: s, Prober: p, Runner: r, Logger: lg,
		Clock: newFakeClock(), WALName: walName, Window: 0, Cap: 15 * time.Second,
	}
}
