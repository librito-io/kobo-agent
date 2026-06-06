package pair

import "time"

// PairRequest is the result of a successful POST /api/pair/request.
type PairRequest struct {
	Code       string // 6-digit user-facing code
	PairingID  string // path param for status polls
	PollSecret string // Bearer on every status GET; returned once, never persisted
	ExpiresIn  int    // seconds (server says 300); we bound by monotonic clock anyway
}

// PairStatus is the result of a GET /api/pair/status/[pairingId] poll.
type PairStatus struct {
	Paired    bool
	Token     string // sk_device_… present only when Paired
	UserEmail string
}

// PollOutcome classifies every status-poll result into exactly one bucket.
// The classifier (classify.go) is total: any input maps to one of these.
type PollOutcome int

const (
	OutcomePaired    PollOutcome = iota // 200 {paired:true} → write token, done
	OutcomeWaiting                      // 200 {paired:false} → keep polling
	OutcomeExpired                      // 410 → show "Code expired" + Retry
	OutcomeTransient                    // 429/503/5xx/net/unknown → backoff, keep polling
	OutcomeFatal                        // 401/404/400 → contract bug, exit nonzero
)

// RequestOutcome classifies a POST /api/pair/request result.
type RequestOutcome int

const (
	ReqOK        RequestOutcome = iota // 200 → proceed to showCode
	ReqTransient                       // 429/503/500/net → backoff, retry (≤ TTL)
	ReqFatal                           // 400 → we sent a bad body; exit nonzero
)

// Button is the user's response on a dialog with action buttons.
type Button int

const (
	ButtonNone   Button = iota // no tap yet (poll the dialog non-blockingly)
	ButtonCancel               // reject=0
	ButtonRetry                // accept=1 on the expired/no-wifi dialog
)

// Client talks to the pairing API. The httptest-mocked impl lives in client.go.
type Client interface {
	// Request POSTs {hardwareId, deviceType:"kobo", deviceModel}. The
	// RequestOutcome tells the caller how to react; retryAfter is the server's
	// Retry-After (0 if absent).
	Request(hardwareID, deviceModel string) (req PairRequest, out RequestOutcome, retryAfter time.Duration, err error)
	// Status GETs status/[pairingId] with Bearer pollSecret on EVERY call.
	Status(pairingID, pollSecret string) (st PairStatus, out PollOutcome, retryAfter time.Duration, err error)
}

// Display is the single live-updating NickelDBus dialog. The qndb impl is in
// display.go; tests use a fake. All methods are best-effort (display is garnish).
type Display interface {
	ShowCode(code string)      // initial dialog: code + Cancel button
	UpdatePaired(email string) // live-update same dialog → "Paired ✓"
	UpdateExpired()            // live-update → "Code expired" + Retry button
	UpdateNoWiFi()             // → "No WiFi" + Retry/Cancel
	UpdateError(msg string)    // → terminal error text
	Poll() Button              // non-blocking read of the latest button tap
	Close()                    // tear the dialog down programmatically
}

// WiFi drives ONLY the silent connect path (CLAUDE.md invariant #7).
type WiFi interface {
	// Connect requests a silent connect and waits up to timeout for the
	// wmNetworkConnected signal. Returns true if connected within the window.
	// MUST NOT ever call the non-silent path or reboot.
	Connect(timeout time.Duration) (connected bool)
}

// (DeviceModel is plumbed via Deps, not Store — it is detected fresh per run.)

// Store persists the two on-disk files under /mnt/onboard/.adds/librito/.
type Store interface {
	// LoadOrCreateHardwareID returns the persisted lowercase UUID, generating +
	// writing one on first run. Reused byte-for-byte forever (idempotent).
	LoadOrCreateHardwareID() (string, error)
	// WriteToken overwrites the token file (re-pair is an intentional overwrite).
	WriteToken(token string) error
}

// Clock abstracts elapsed time so the TTL bound is testable and uses MONOTONIC
// time, never the device wall clock (which is unreliable on the Kobo).
type Clock interface {
	Now() time.Time        // monotonic reference
	Sleep(d time.Duration) // backoff / poll cadence wait
}
