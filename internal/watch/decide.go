package watch

// action is the watcher's decision after a debounced WAL event.
type action int

const (
	actionSkipNoGrowth action = iota // signature did not grow → absorb, advance baseline
	actionSkipNotReady               // grew but offline → hold baseline, retry later
	actionSync                       // grew and connected → trigger the sync
)

// decide maps (prev signature, cur signature, connectivity) to the loop's action.
// Pure: no growth short-circuits regardless of connectivity; growth requires
// readiness to sync (otherwise the udev autosync up-edge syncs it later).
func decide(prev, cur Signature, ready bool) action {
	if !grew(prev, cur) {
		return actionSkipNoGrowth
	}
	if !ready {
		return actionSkipNotReady
	}
	return actionSync
}
