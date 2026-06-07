package watch

import "time"

// debounceWait returns how long to wait from now before a debounce burst is due,
// given the burst's first event time, its most recent event time (last), the
// silence window, and the max-wait cap. It fires on whichever comes first:
// window-of-silence after the last event, or maxWait after the first event (the
// cap, so a never-silent stream can't starve forever). A value <= 0 means due now.
func debounceWait(first, last, now time.Time, window, maxWait time.Duration) time.Duration {
	bySilence := last.Add(window).Sub(now)
	byCap := first.Add(maxWait).Sub(now)
	if byCap < bySilence {
		return byCap
	}
	return bySilence
}
