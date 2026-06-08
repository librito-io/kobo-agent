package autosync

import "testing"

func TestExitCode(t *testing.T) {
	cases := map[Outcome]int{
		OutcomeSynced: 0, OutcomeDedup: 0, OutcomeUnpaired: 0,
		OutcomeOffline: 1, OutcomeError: 1, OutcomeLockErr: 1,
	}
	for o, want := range cases {
		if got := o.ExitCode(); got != want {
			t.Errorf("Outcome(%d).ExitCode() = %d, want %d", o, got, want)
		}
	}
}

func TestShouldToast_Allowlist(t *testing.T) {
	allow := []string{"home"}
	if !ShouldToast("home", allow) {
		t.Error("home is allow-listed → should toast")
	}
	if ShouldToast("reading", allow) {
		t.Error("reading must never toast")
	}
	if ShouldToast("some_new_view", allow) {
		t.Error("unknown view must fail safe → no toast")
	}
	if ShouldToast("home", nil) {
		t.Error("empty allowlist → never toast")
	}
}
