package pair

// ClassifyStatus maps a GET /status result to exactly one PollOutcome.
// Per the verified web taxonomy (spec §"Verified HTTP taxonomy"):
//
//	200 paired:true → Paired; 200 paired:false → Waiting; 410 → Expired;
//	401/404 → Fatal (our contract bug); everything else (429/503/5xx/transport/
//	unknown) → Transient — safe because the loop bounds Transient by TTL.
func ClassifyStatus(code int, paired, transport bool) PollOutcome {
	if transport {
		return OutcomeTransient
	}
	switch code {
	case 200:
		if paired {
			return OutcomePaired
		}
		return OutcomeWaiting
	case 410:
		return OutcomeExpired
	case 401, 404:
		return OutcomeFatal
	default:
		return OutcomeTransient
	}
}

// ClassifyRequest maps a POST /request result to a RequestOutcome.
//
//	200 → OK; 400 → Fatal (we send a valid v4, so 400 is our bug);
//	429/503/500/transport/unknown → Transient.
func ClassifyRequest(code int, transport bool) RequestOutcome {
	if transport {
		return ReqTransient
	}
	switch code {
	case 200:
		return ReqOK
	case 400:
		return ReqFatal
	default:
		return ReqTransient
	}
}
