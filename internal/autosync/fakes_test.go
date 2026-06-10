package autosync

import "time"

// fakeClock: monotonic time the test controls; Sleep advances it (no real wait).
type fakeClock struct{ t time.Time }

func newFakeClock() *fakeClock             { return &fakeClock{t: time.Date(2026, 6, 7, 19, 42, 0, 0, time.UTC)} }
func (c *fakeClock) Now() time.Time        { return c.t }
func (c *fakeClock) Sleep(d time.Duration) { c.t = c.t.Add(d) }

// fakeProber: returns a scripted sequence of snapshots; the last repeats.
type fakeProber struct {
	snaps []Snapshot
	calls int
}

func (p *fakeProber) Probe() Snapshot {
	i := p.calls
	if i >= len(p.snaps) {
		i = len(p.snaps) - 1
	}
	p.calls++
	return p.snaps[i]
}

// fakeLocker: scripted lock outcome; records acquire/release.
type fakeLocker struct {
	ok       bool
	err      error
	acquired bool
	released bool
}

func (l *fakeLocker) TryLock() (bool, func(), error) {
	if l.err != nil {
		return false, func() {}, l.err
	}
	if !l.ok {
		return false, func() {}, nil
	}
	l.acquired = true
	return true, func() { l.released = true }, nil
}

// fakeConfig: in-memory token + baseURL.
type fakeConfig struct {
	token   string
	baseURL string
}

func (c fakeConfig) Token() string   { return c.token }
func (c fakeConfig) BaseURL() string { return c.baseURL }

// fakeSyncer: scripted result; records the args it was called with.
type fakeSyncer struct {
	imported int
	books    int
	err      error
	calls    int
	gotURL   string
	gotToken string
}

func (s *fakeSyncer) Sync(baseURL, token string) (int, int, error) {
	s.calls++
	s.gotURL = baseURL
	s.gotToken = token
	return s.imported, s.books, s.err
}

// fakeLogger: records every line.
type fakeLogger struct{ lines []string }

func (l *fakeLogger) Log(line string) { l.lines = append(l.lines, line) }

// fakeRecordStore records the (outcome, count) pairs Record was called with, and
// returns a scripted lastCount as the prior recorded count.
type fakeRecordStore struct {
	got       []Outcome
	counts    []int
	lastCount int
}

func (s *fakeRecordStore) Record(o Outcome, count int) {
	s.got = append(s.got, o)
	s.counts = append(s.counts, count)
}
func (s *fakeRecordStore) LastCount() int { return s.lastCount }

// fakeViewProber returns a scripted view string.
type fakeViewProber struct{ view string }

func (p fakeViewProber) CurrentView() string { return p.view }

// countingViewProber counts probes — to prove the grow-gate short-circuits the
// view probe (a qndb exec) when nothing grew.
type countingViewProber struct {
	view  string
	calls int
}

func (p *countingViewProber) CurrentView() string { p.calls++; return p.view }

// fakeToaster records toast calls.
type fakeToaster struct{ mains []string }

func (t *fakeToaster) Toast(main, sub string) { t.mains = append(t.mains, main) }
