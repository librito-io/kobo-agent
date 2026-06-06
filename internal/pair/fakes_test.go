package pair

import "time"

// --- fakeClock: monotonic time the test controls; Sleep advances it. ---
type fakeClock struct{ t time.Time }

func newFakeClock() *fakeClock             { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }
func (c *fakeClock) Now() time.Time        { return c.t }
func (c *fakeClock) Sleep(d time.Duration) { c.t = c.t.Add(d) } // no real sleep; just advance

// --- fakeStore: in-memory hardware-id + token, records token writes. ---
type fakeStore struct {
	hwid        string
	hwidCalls   int
	token       string
	tokenWrites int
}

func (s *fakeStore) LoadOrCreateHardwareID() (string, error) {
	s.hwidCalls++
	if s.hwid == "" {
		s.hwid = "abababab-abab-4bab-8bab-abababababab"
	}
	return s.hwid, nil
}
func (s *fakeStore) WriteToken(t string) error { s.token = t; s.tokenWrites++; return nil }

// --- fakeWiFi: scripted connect result(s). ---
type fakeWiFi struct {
	results []bool // consumed per Connect call; default true when exhausted
	calls   int
}

func (w *fakeWiFi) Connect(timeout time.Duration) bool {
	r := true
	if w.calls < len(w.results) {
		r = w.results[w.calls]
	}
	w.calls++
	return r
}

// fakeDisplayState identifies which dialog is currently showing, so the fake can
// return button taps scoped to that dialog. The real qndb impl latches the last
// tap regardless; modelling per-state queues keeps test button accounting from
// drifting when a state (expired / no-wifi) polls in addition to the poll loop's
// top-of-cycle cancel check.
type fakeDisplayState int

const (
	stateNone fakeDisplayState = iota
	stateCode
	stateExpired
	stateNoWiFi
)

// --- fakeDisplay: records the last state shown + per-state queued buttons. ---
//
// buttons is keyed by the dialog state active when Poll() is called: a Retry
// queued under stateExpired is only returned while the expired dialog shows, so
// the poll loop's cancel check (which runs under stateCode) won't swallow it.
type fakeDisplay struct {
	shownCode   string
	pairedEmail string
	expired     bool
	noWiFi      bool
	errMsg      string
	closed      bool

	state     fakeDisplayState
	buttons   map[fakeDisplayState][]Button // per-state tap script
	pollCalls map[fakeDisplayState]int      // per-state cursor
}

func (d *fakeDisplay) ShowCode(code string)      { d.shownCode = code; d.state = stateCode }
func (d *fakeDisplay) UpdatePaired(email string) { d.pairedEmail = email }
func (d *fakeDisplay) UpdateExpired()            { d.expired = true; d.state = stateExpired }
func (d *fakeDisplay) UpdateNoWiFi()             { d.noWiFi = true; d.state = stateNoWiFi }
func (d *fakeDisplay) UpdateError(msg string)    { d.errMsg = msg }
func (d *fakeDisplay) Close()                    { d.closed = true }

func (d *fakeDisplay) Poll() Button {
	if d.pollCalls == nil {
		d.pollCalls = map[fakeDisplayState]int{}
	}
	i := d.pollCalls[d.state]
	d.pollCalls[d.state] = i + 1
	q := d.buttons[d.state]
	if i < len(q) {
		return q[i]
	}
	return ButtonNone
}

// --- fakeClient: scripted request + status outcomes. ---
type reqStep struct {
	pr  PairRequest
	out RequestOutcome
	ra  time.Duration
}
type statusStep struct {
	st  PairStatus
	out PollOutcome
	ra  time.Duration
}
type fakeClient struct {
	reqSteps    []reqStep
	reqCalls    int
	statusSteps []statusStep
	statusCalls int
	// records pollSecret seen on each status call, to assert it's threaded.
	seenSecrets []string
	// records (hardwareID, deviceModel) seen on each request call.
	seenHWIDs  []string
	seenModels []string
}

func (c *fakeClient) Request(hardwareID, deviceModel string) (PairRequest, RequestOutcome, time.Duration, error) {
	if len(c.reqSteps) == 0 {
		panic("fakeClient.Request called but reqSteps is empty (test wired no request)")
	}
	c.seenHWIDs = append(c.seenHWIDs, hardwareID)
	c.seenModels = append(c.seenModels, deviceModel)
	s := c.reqSteps[min(c.reqCalls, len(c.reqSteps)-1)]
	c.reqCalls++
	return s.pr, s.out, s.ra, nil
}
func (c *fakeClient) Status(pairingID, pollSecret string) (PairStatus, PollOutcome, time.Duration, error) {
	if len(c.statusSteps) == 0 {
		panic("fakeClient.Status called but statusSteps is empty (test wired no status)")
	}
	c.seenSecrets = append(c.seenSecrets, pollSecret)
	s := c.statusSteps[min(c.statusCalls, len(c.statusSteps)-1)]
	c.statusCalls++
	return s.st, s.out, s.ra, nil
}
