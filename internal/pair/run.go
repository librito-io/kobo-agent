package pair

import "time"

// Deps wires Run to its impure edges (fakes in tests, real impls in main).
type Deps struct {
	Client  Client
	Display Display
	WiFi    WiFi
	Store   Store
	Clock   Clock

	DeviceModel    string        // human-legible model sent on every pair request (devices.model)
	BaseURL        string        // backend the device paired against; persisted for autosync (Step 3)
	WiFiTimeout    time.Duration // bounded wait for wmNetworkConnected (~20s)
	PollEvery      time.Duration // status poll cadence (~5s)
	CodeTTL        time.Duration // monotonic lifetime of a code (300s)
	DecisionWindow time.Duration // how long to wait for a Retry/Cancel tap on a
	// No-WiFi / expired dialog before giving up (defaults to CodeTTL if zero)
}

// ResultStatus is Run's terminal state, mapped to an exit code by main.
type ResultStatus int

const (
	ResultPaired    ResultStatus = iota // success
	ResultCancelled                     // user tapped Cancel (or No-WiFi Cancel)
	ResultExpired                       // code expired, user did not retry
	ResultFatal                         // contract bug (401/404/400) → nonzero exit
)

// Result is what Run returns.
type Result struct {
	Status ResultStatus
}

// Run executes the pairing flow:
//
//	ensureWiFi → request → showCode → poll-loop → writeToken → showPaired → done
//
// All logic is pure over the Deps interfaces; the loop is bounded by a MONOTONIC
// elapsed measurement (Clock), never the device wall clock. Display calls are
// best-effort garnish; the loop never depends on them succeeding.
func Run(d Deps) Result {
	defer d.Display.Close()

	hwid, err := d.Store.LoadOrCreateHardwareID()
	if err != nil {
		d.Display.UpdateError("pairing failed")
		return Result{Status: ResultFatal}
	}

	// ensureWiFi — silent path only, bounded, with a No-WiFi Retry/Cancel dialog.
	for !d.WiFi.Connect(d.WiFiTimeout) {
		d.Display.UpdateNoWiFi()
		// Wait for an actual tap: the qndb Poll() is non-blocking and returns
		// ButtonNone until the user acts, so we must not read it once and bail —
		// that would make the No-WiFi dialog unreachable on hardware.
		switch waitButton(d) {
		case ButtonRetry:
			continue // re-attempt the silent connect
		default: // Cancel or the decision window lapsed → give up
			return Result{Status: ResultCancelled}
		}
	}

	// request → poll, with 410-Retry re-entering request.
	for {
		pr, reqOut, reqRA, _ := d.Client.Request(hwid, d.DeviceModel)
		switch reqOut {
		case ReqOK:
			// fall through to poll
		case ReqFatal:
			d.Display.UpdateError("pairing failed")
			return Result{Status: ResultFatal}
		default: // ReqTransient
			d.Clock.Sleep(backoff(reqRA, d.PollEvery))
			continue
		}

		d.Display.ShowCode(pr.Code)
		t0 := d.Clock.Now()

		res, retry := pollLoop(d, pr, t0)
		if retry {
			continue // user tapped Retry on expiry → re-request
		}
		return res
	}
}

// pollLoop polls status until a terminal outcome. Returns (result, retry); when
// retry is true the caller re-enters request (410 + user Retry).
func pollLoop(d Deps, pr PairRequest, t0 time.Time) (Result, bool) {
	for {
		// Cancel check before each network call.
		if d.Display.Poll() == ButtonCancel {
			return Result{Status: ResultCancelled}, false
		}

		// Monotonic TTL bound — independent of the device wall clock. This is a
		// hard client-side guard, not the user-interactive expiry: the server 410
		// (OutcomeExpired below) is what offers Retry. Reaching the local TTL means
		// the code is certainly dead, so we surface "expired" and stop.
		if d.Clock.Now().Sub(t0) > d.CodeTTL {
			d.Display.UpdateExpired()
			return Result{Status: ResultExpired}, false
		}

		st, out, ra, _ := d.Client.Status(pr.PairingID, pr.PollSecret)
		switch out {
		case OutcomePaired:
			// Persist the paired backend URL BEFORE the token: a paired device
			// (token present) is then guaranteed to also have the url file, and a
			// url-write failure aborts before any token is written (Step 3 coupling).
			if err := d.Store.WriteURL(d.BaseURL); err != nil {
				d.Display.UpdateError("could not save backend")
				return Result{Status: ResultFatal}, false
			}
			if err := d.Store.WriteToken(st.Token); err != nil {
				d.Display.UpdateError("could not save token")
				return Result{Status: ResultFatal}, false
			}
			d.Display.UpdatePaired(st.UserEmail)
			d.Clock.Sleep(d.PollEvery) // let "Paired ✓" linger a beat before Close
			return Result{Status: ResultPaired}, false

		case OutcomeWaiting:
			d.Clock.Sleep(d.PollEvery)

		case OutcomeTransient:
			d.Clock.Sleep(backoff(ra, d.PollEvery))

		case OutcomeExpired:
			d.Display.UpdateExpired()
			// Wait for a real tap (Poll() is non-blocking and starts at
			// ButtonNone) before deciding retry-vs-stop; reading it once would
			// always miss the user on hardware.
			if waitButton(d) == ButtonRetry {
				return Result{}, true // re-request
			}
			return Result{Status: ResultExpired}, false

		case OutcomeFatal:
			d.Display.UpdateError("pairing failed")
			return Result{Status: ResultFatal}, false
		}
	}
}

// backoff honors a server Retry-After when present, else falls back to the poll
// cadence. (A richer exponential backoff is unnecessary: the TTL already bounds
// the loop, and the cadence keeps us under the server's 1/3s cap.)
func backoff(retryAfter, fallback time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	return fallback
}

// waitButton blocks (via the injected Clock) until the user taps a button on the
// current dialog or the decision window lapses. The qndb Display.Poll() is
// non-blocking and returns ButtonNone until a tap latches, so the interactive
// No-WiFi / expired prompts must poll it on the cadence rather than read it once
// — a single read would always miss the user and silently treat every prompt as
// "no tap". A lapsed window returns ButtonNone (caller treats it as Cancel/stop),
// bounding the wait so a walked-away device can't sit on a dialog forever.
func waitButton(d Deps) Button {
	window := d.DecisionWindow
	if window <= 0 {
		window = d.CodeTTL
	}
	deadline := d.Clock.Now().Add(window)
	for {
		if b := d.Display.Poll(); b != ButtonNone {
			return b
		}
		if !d.Clock.Now().Before(deadline) {
			return ButtonNone
		}
		d.Clock.Sleep(d.PollEvery)
	}
}
