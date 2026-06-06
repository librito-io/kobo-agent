package pair

import (
	"testing"
	"time"
)

// helper: default deps wired to fakes, overridable per test.
func deps(c *fakeClient, d *fakeDisplay, w *fakeWiFi, s *fakeStore, clk *fakeClock) Deps {
	return Deps{
		Client: c, Display: d, WiFi: w, Store: s, Clock: clk,
		DeviceModel:    "Kobo Libra Colour",
		WiFiTimeout:    20 * time.Second,
		PollEvery:      5 * time.Second,
		CodeTTL:        300 * time.Second,
		DecisionWindow: 60 * time.Second, // match production intent (not the CodeTTL fallback)
	}
}

func TestRun_HappyPath(t *testing.T) {
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{Code: "482913", PairingID: "pid-1", PollSecret: "ps-1", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{out: OutcomeWaiting}, {st: PairStatus{Paired: true, Token: "sk_device_x", UserEmail: "a@b.co"}, out: OutcomePaired}},
	}
	d, w, s, clk := &fakeDisplay{}, &fakeWiFi{}, &fakeStore{}, newFakeClock()
	res := Run(deps(c, d, w, s, clk))

	if res.Status != ResultPaired {
		t.Fatalf("result = %v, want ResultPaired", res.Status)
	}
	if d.shownCode != "482913" {
		t.Fatalf("code shown = %q", d.shownCode)
	}
	if s.token != "sk_device_x" || s.tokenWrites != 1 {
		t.Fatalf("token not written exactly once: %q writes=%d", s.token, s.tokenWrites)
	}
	if d.pairedEmail != "a@b.co" {
		t.Fatalf("paired email = %q", d.pairedEmail)
	}
	if !d.closed {
		t.Fatal("dialog not closed at end")
	}
	// pollSecret threaded on EVERY status call.
	for i, sec := range c.seenSecrets {
		if sec != "ps-1" {
			t.Fatalf("status call %d used pollSecret %q, want ps-1", i, sec)
		}
	}
	// deviceModel from Deps threaded into the request.
	if len(c.seenModels) != 1 || c.seenModels[0] != "Kobo Libra Colour" {
		t.Fatalf("request deviceModel = %v, want [Kobo Libra Colour]", c.seenModels)
	}
}

func TestRun_HardwareIDReusedNotRegenerated(t *testing.T) {
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{st: PairStatus{Paired: true, Token: "tok"}, out: OutcomePaired}},
	}
	s := &fakeStore{hwid: "abababab-abab-4bab-8bab-abababababab"}
	Run(deps(c, &fakeDisplay{}, &fakeWiFi{}, s, newFakeClock()))
	if s.hwidCalls != 1 {
		t.Fatalf("hardwareID loaded %d times, want exactly 1", s.hwidCalls)
	}
}

func TestRun_NoWiFi_ShowsDialogAndCancel(t *testing.T) {
	// WiFi never connects; user taps Cancel on the No-WiFi dialog.
	w := &fakeWiFi{results: []bool{false}}
	d := &fakeDisplay{buttons: map[fakeDisplayState][]Button{stateNoWiFi: {ButtonCancel}}}
	c := &fakeClient{} // never reached
	res := Run(deps(c, d, w, &fakeStore{}, newFakeClock()))
	if res.Status != ResultCancelled {
		t.Fatalf("result = %v, want ResultCancelled", res.Status)
	}
	if !d.noWiFi {
		t.Fatal("No-WiFi dialog not shown")
	}
	if c.reqCalls != 0 {
		t.Fatal("must not call /request when WiFi never connected")
	}
}

func TestRun_NoWiFi_RetryThenConnect(t *testing.T) {
	// First connect fails → No-WiFi dialog → user taps Retry → second connect ok.
	w := &fakeWiFi{results: []bool{false, true}}
	d := &fakeDisplay{buttons: map[fakeDisplayState][]Button{stateNoWiFi: {ButtonRetry}}}
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{st: PairStatus{Paired: true, Token: "tok"}, out: OutcomePaired}},
	}
	res := Run(deps(c, d, w, &fakeStore{}, newFakeClock()))
	if res.Status != ResultPaired {
		t.Fatalf("result = %v, want ResultPaired after retry", res.Status)
	}
	if w.calls != 2 {
		t.Fatalf("WiFi.Connect called %d times, want 2 (fail then retry)", w.calls)
	}
}

func TestRun_Expired_ShowsExpiredThenRetryRerequests(t *testing.T) {
	// Poll returns expired; user taps Retry → re-request → then paired.
	c := &fakeClient{
		reqSteps: []reqStep{
			{pr: PairRequest{Code: "111111", PollSecret: "ps1", ExpiresIn: 300}, out: ReqOK},
			{pr: PairRequest{Code: "222222", PollSecret: "ps2", ExpiresIn: 300}, out: ReqOK},
		},
		statusSteps: []statusStep{
			{out: OutcomeExpired},
			{st: PairStatus{Paired: true, Token: "tok"}, out: OutcomePaired},
		},
	}
	d := &fakeDisplay{buttons: map[fakeDisplayState][]Button{stateExpired: {ButtonRetry}}}
	res := Run(deps(c, d, &fakeWiFi{}, &fakeStore{}, newFakeClock()))
	if res.Status != ResultPaired {
		t.Fatalf("result = %v, want ResultPaired", res.Status)
	}
	if !d.expired {
		t.Fatal("expired dialog not shown")
	}
	if c.reqCalls != 2 {
		t.Fatalf("re-request count = %d, want 2 (initial + retry)", c.reqCalls)
	}
	// second status poll must use the NEW pollSecret from the re-request.
	last := c.seenSecrets[len(c.seenSecrets)-1]
	if last != "ps2" {
		t.Fatalf("post-retry poll used secret %q, want ps2", last)
	}
}

func TestRun_Expired_NoAutoRestart(t *testing.T) {
	// On expiry with NO button tap (ButtonNone), the loop must STOP at expired —
	// never silently re-request (burns the 5/5min hwId limit).
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{out: OutcomeExpired}},
	}
	d := &fakeDisplay{} // no buttons → ButtonNone forever
	res := Run(deps(c, d, &fakeWiFi{}, &fakeStore{}, newFakeClock()))
	if res.Status != ResultExpired {
		t.Fatalf("result = %v, want ResultExpired (no auto-restart)", res.Status)
	}
	if c.reqCalls != 1 {
		t.Fatalf("re-request count = %d, want 1 (no auto-restart)", c.reqCalls)
	}
}

func TestRun_TransientBackoffThenRecover(t *testing.T) {
	// 429 transient, then paired. Loop must keep polling (not exit), and advance
	// the clock by the backoff each time (asserted via clock).
	clk := newFakeClock()
	start := clk.Now()
	c := &fakeClient{
		reqSteps: []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{
			{out: OutcomeTransient, ra: 7 * time.Second},
			{st: PairStatus{Paired: true, Token: "tok"}, out: OutcomePaired},
		},
	}
	res := Run(deps(c, &fakeDisplay{}, &fakeWiFi{}, &fakeStore{}, clk))
	if res.Status != ResultPaired {
		t.Fatalf("result = %v, want ResultPaired", res.Status)
	}
	if !clk.Now().After(start) {
		t.Fatal("clock did not advance — backoff not applied")
	}
}

func TestRun_FatalExitsNonzeroWithoutSpinning(t *testing.T) {
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{out: OutcomeFatal}},
	}
	d := &fakeDisplay{}
	res := Run(deps(c, d, &fakeWiFi{}, &fakeStore{}, newFakeClock()))
	if res.Status != ResultFatal {
		t.Fatalf("result = %v, want ResultFatal", res.Status)
	}
	if c.statusCalls != 1 {
		t.Fatalf("polled %d times on fatal, want exactly 1 (no spin)", c.statusCalls)
	}
	if d.errMsg == "" {
		t.Fatal("error dialog not shown on fatal")
	}
}

func TestRun_RequestFatalExits(t *testing.T) {
	c := &fakeClient{reqSteps: []reqStep{{out: ReqFatal}}}
	res := Run(deps(c, &fakeDisplay{}, &fakeWiFi{}, &fakeStore{}, newFakeClock()))
	if res.Status != ResultFatal {
		t.Fatalf("result = %v, want ResultFatal on request 400", res.Status)
	}
}

func TestRun_CancelDuringPoll(t *testing.T) {
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{out: OutcomeWaiting}, {out: OutcomeWaiting}},
	}
	// cancel on 2nd poll cycle (code dialog is the active state during polling).
	d := &fakeDisplay{buttons: map[fakeDisplayState][]Button{stateCode: {ButtonNone, ButtonCancel}}}
	res := Run(deps(c, d, &fakeWiFi{}, &fakeStore{}, newFakeClock()))
	if res.Status != ResultCancelled {
		t.Fatalf("result = %v, want ResultCancelled", res.Status)
	}
}

func TestRun_NoWiFi_WaitsForTapAcrossNonePolls(t *testing.T) {
	// Regression guard: the qndb Poll() is non-blocking and returns ButtonNone
	// until the user taps. The No-WiFi prompt must keep polling, not read once
	// and treat the absent tap as Cancel. Here the tap (Retry) only lands on the
	// 3rd poll; connect then succeeds. A read-once impl would exit Cancelled.
	w := &fakeWiFi{results: []bool{false, true}}
	d := &fakeDisplay{buttons: map[fakeDisplayState][]Button{
		stateNoWiFi: {ButtonNone, ButtonNone, ButtonRetry},
	}}
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{st: PairStatus{Paired: true, Token: "tok"}, out: OutcomePaired}},
	}
	res := Run(deps(c, d, w, &fakeStore{}, newFakeClock()))
	if res.Status != ResultPaired {
		t.Fatalf("result = %v, want ResultPaired (tap on 3rd poll must be honored)", res.Status)
	}
}

func TestRun_NoWiFi_DecisionWindowLapsesToCancel(t *testing.T) {
	// If the user never taps, the bounded decision window must lapse and exit
	// (not spin forever). The fake clock advances on each Sleep until the window
	// is exhausted.
	w := &fakeWiFi{results: []bool{false}} // never connects
	d := &fakeDisplay{}                    // no taps ever → ButtonNone forever
	res := Run(deps(c0(), d, w, &fakeStore{}, newFakeClock()))
	if res.Status != ResultCancelled {
		t.Fatalf("result = %v, want ResultCancelled when the window lapses", res.Status)
	}
}

// c0 is an empty client used by tests where /request must never be reached.
func c0() *fakeClient { return &fakeClient{} }

func TestRun_TTLBoundUsesMonotonicClock(t *testing.T) {
	// Always-waiting status; the loop must terminate when elapsed > CodeTTL,
	// driven purely by the fake clock advancing on Sleep (no wall time).
	c := &fakeClient{
		reqSteps:    []reqStep{{pr: PairRequest{PollSecret: "ps", ExpiresIn: 300}, out: ReqOK}},
		statusSteps: []statusStep{{out: OutcomeWaiting}}, // repeats forever
	}
	res := Run(deps(c, &fakeDisplay{}, &fakeWiFi{}, &fakeStore{}, newFakeClock()))
	if res.Status != ResultExpired {
		t.Fatalf("result = %v, want ResultExpired at TTL", res.Status)
	}
}
