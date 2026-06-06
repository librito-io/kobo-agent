package pair

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxBody bounds how much of a response we read into memory — the device is
// memory-constrained, and both endpoints are our own backend returning tiny
// JSON. Matches internal/sync/client.go's 64 KiB cap.
const maxBody = 64 * 1024

// httpClient is the impure Client edge. Like internal/sync, it relies on Go's
// own TLS stack + CA roots, not the Kobo's stale system store.
type httpClient struct {
	baseURL string
	hc      *http.Client
}

// NewHTTPClient builds a Client against baseURL (e.g. https://librito.io) with
// a per-request timeout.
func NewHTTPClient(baseURL string, timeout time.Duration) Client {
	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		hc:      &http.Client{Timeout: timeout},
	}
}

func (c *httpClient) Request(hardwareID string) (PairRequest, RequestOutcome, time.Duration, error) {
	body, _ := json.Marshal(map[string]string{
		"hardwareId": hardwareID,
		"deviceType": "kobo", // forward-compat; web ignores it today (web devices.type issue)
	})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/pair/request", bytes.NewReader(body))
	if err != nil {
		return PairRequest{}, ReqTransient, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return PairRequest{}, ClassifyRequest(0, true), 0, nil
	}
	defer resp.Body.Close()
	ra := parseRetryAfter(resp.Header.Get("Retry-After"))

	out := ClassifyRequest(resp.StatusCode, false)
	if out != ReqOK {
		return PairRequest{}, out, ra, nil
	}
	var pr struct {
		Code       string `json:"code"`
		PairingID  string `json:"pairingId"`
		PollSecret string `json:"pollSecret"`
		ExpiresIn  int    `json:"expiresIn"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&pr); err != nil {
		return PairRequest{}, ReqTransient, ra, nil // unparseable 200 → treat as transient
	}
	return PairRequest{Code: pr.Code, PairingID: pr.PairingID, PollSecret: pr.PollSecret, ExpiresIn: pr.ExpiresIn}, ReqOK, ra, nil
}

func (c *httpClient) Status(pairingID, pollSecret string) (PairStatus, PollOutcome, time.Duration, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/pair/status/"+pairingID, nil)
	if err != nil {
		return PairStatus{}, OutcomeTransient, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+pollSecret) // pollSecret on EVERY poll

	resp, err := c.hc.Do(req)
	if err != nil {
		return PairStatus{}, ClassifyStatus(0, false, true), 0, nil
	}
	defer resp.Body.Close()
	ra := parseRetryAfter(resp.Header.Get("Retry-After"))

	if resp.StatusCode != 200 {
		return PairStatus{}, ClassifyStatus(resp.StatusCode, false, false), ra, nil
	}
	var st struct {
		Paired    bool   `json:"paired"`
		Token     string `json:"token"`
		UserEmail string `json:"userEmail"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&st); err != nil {
		return PairStatus{}, OutcomeTransient, ra, nil
	}
	ps := PairStatus{Paired: st.Paired, Token: st.Token, UserEmail: st.UserEmail}
	return ps, ClassifyStatus(200, st.Paired, false), ra, nil
}

// parseRetryAfter reads the integer-seconds form of Retry-After; 0 if absent or
// non-numeric (the HTTP-date form is not used by the web limiter).
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}
