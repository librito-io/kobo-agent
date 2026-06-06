package pair

import "testing"

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		name      string
		code      int
		paired    bool // body said paired:true (only meaningful at 200)
		transport bool // transport error (code irrelevant)
		want      PollOutcome
	}{
		{"200 paired", 200, true, false, OutcomePaired},
		{"200 not yet", 200, false, false, OutcomeWaiting},
		{"410 expired", 410, false, false, OutcomeExpired},
		{"429 rate limited", 429, false, false, OutcomeTransient},
		{"503 limiter down", 503, false, false, OutcomeTransient},
		{"500 server blip", 500, false, false, OutcomeTransient},
		{"401 our bug", 401, false, false, OutcomeFatal},
		{"404 bad pairingId", 404, false, false, OutcomeFatal},
		{"transport error", 0, false, true, OutcomeTransient},
		{"unknown 418 → transient", 418, false, false, OutcomeTransient},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyStatus(c.code, c.paired, c.transport)
			if got != c.want {
				t.Fatalf("ClassifyStatus(%d,paired=%v,transport=%v) = %v, want %v",
					c.code, c.paired, c.transport, got, c.want)
			}
		})
	}
}

func TestClassifyRequest(t *testing.T) {
	cases := []struct {
		name      string
		code      int
		transport bool
		want      RequestOutcome
	}{
		{"200 ok", 200, false, ReqOK},
		{"429 quota", 429, false, ReqTransient},
		{"503 limiter down", 503, false, ReqTransient},
		{"500 codegen failed", 500, false, ReqTransient},
		{"400 bad body our bug", 400, false, ReqFatal},
		{"transport error", 0, true, ReqTransient},
		{"unknown 418 → transient", 418, false, ReqTransient},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyRequest(c.code, c.transport)
			if got != c.want {
				t.Fatalf("ClassifyRequest(%d,transport=%v) = %v, want %v",
					c.code, c.transport, got, c.want)
			}
		})
	}
}
