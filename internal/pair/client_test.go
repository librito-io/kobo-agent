package pair

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Request_ShapeAndOK(t *testing.T) {
	var gotPath, gotCT, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"code":"482913","pairingId":"pid-1","pollSecret":"ps-1","expiresIn":300}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, 5*time.Second)
	pr, out, _, err := c.Request("abababab-abab-4bab-8bab-abababababab", "Kobo Libra Colour")
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if out != ReqOK {
		t.Fatalf("outcome = %v, want ReqOK", out)
	}
	if pr.Code != "482913" || pr.PairingID != "pid-1" || pr.PollSecret != "ps-1" || pr.ExpiresIn != 300 {
		t.Fatalf("parsed PairRequest wrong: %+v", pr)
	}
	if gotPath != "/api/pair/request" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "" {
		t.Fatalf("request must be unauthenticated, got auth %q", gotAuth)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Fatalf("content-type = %q", gotCT)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, gotBody)
	}
	if body["hardwareId"] != "abababab-abab-4bab-8bab-abababababab" {
		t.Fatalf("hardwareId wrong: %s", gotBody)
	}
	if body["deviceType"] != "kobo" { // closed set; non-"kobo" coerces to papers3 server-side
		t.Fatalf("deviceType should be kobo: %s", gotBody)
	}
	if body["deviceModel"] != "Kobo Libra Colour" {
		t.Fatalf("deviceModel should round-trip: %s", gotBody)
	}
}

func TestClient_Request_EscapesModelWithQuotes(t *testing.T) {
	// A model string with a quote/backslash must not break the JSON body —
	// json.Marshal escapes it; a string-built body would 400 the pairing.
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"1","pairingId":"p","pollSecret":"s","expiresIn":300}`))
	}))
	defer srv.Close()

	model := `Kobo "Libra" \ Colour`
	if _, out, _, err := NewHTTPClient(srv.URL, 5*time.Second).Request("id", model); err != nil || out != ReqOK {
		t.Fatalf("Request: out=%v err=%v", out, err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body not valid JSON after escaping: %v (%s)", err, gotBody)
	}
	if body["deviceModel"] != model {
		t.Fatalf("deviceModel not round-tripped: got %v", body["deviceModel"])
	}
}

func TestClient_Request_FatalAndTransient(t *testing.T) {
	for _, tc := range []struct {
		code int
		want RequestOutcome
	}{{400, ReqFatal}, {429, ReqTransient}, {503, ReqTransient}, {500, ReqTransient}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		_, out, _, _ := NewHTTPClient(srv.URL, 5*time.Second).Request("id", "Kobo Libra Colour")
		srv.Close()
		if out != tc.want {
			t.Fatalf("code %d → %v, want %v", tc.code, out, tc.want)
		}
	}
}

func TestClient_Status_BearerAndPaired(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"paired":true,"token":"sk_device_abc","userEmail":"a@b.co"}`))
	}))
	defer srv.Close()

	st, out, _, err := NewHTTPClient(srv.URL, 5*time.Second).Status("pid-1", "ps-1")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if out != OutcomePaired || !st.Paired || st.Token != "sk_device_abc" || st.UserEmail != "a@b.co" {
		t.Fatalf("status wrong: out=%v st=%+v", out, st)
	}
	if gotPath != "/api/pair/status/pid-1" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer ps-1" { // pollSecret bearer on EVERY poll
		t.Fatalf("auth = %q, want Bearer ps-1", gotAuth)
	}
}

func TestClient_Status_WaitingExpiredFatal(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
		want PollOutcome
	}{
		{200, `{"paired":false}`, OutcomeWaiting},
		{410, `{"error":"code_expired"}`, OutcomeExpired},
		{401, ``, OutcomeFatal},
		{404, ``, OutcomeFatal},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, out, _, _ := NewHTTPClient(srv.URL, 5*time.Second).Status("pid", "ps")
		srv.Close()
		if out != tc.want {
			t.Fatalf("status code %d → %v, want %v", tc.code, out, tc.want)
		}
	}
}

func TestClient_Status_RetryAfterParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	_, out, ra, _ := NewHTTPClient(srv.URL, 5*time.Second).Status("pid", "ps")
	if out != OutcomeTransient {
		t.Fatalf("429 → %v, want Transient", out)
	}
	if ra != 7*time.Second {
		t.Fatalf("retryAfter = %v, want 7s", ra)
	}
}
