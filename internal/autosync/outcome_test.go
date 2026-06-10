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

func TestGrew(t *testing.T) {
	cases := []struct {
		name        string
		count, last int
		want        bool
	}{
		{"first sync from zero", 1, 0, true},
		{"nothing ever, nothing now", 0, 0, false},
		{"offline backlog flushed", 13, 10, true},
		{"resend of unchanged set", 13, 13, false},
		{"single new highlight", 6, 5, true},
		{"device-side delete shrinks set", 5, 10, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Grew(c.count, c.last); got != c.want {
				t.Errorf("Grew(%d, %d) = %v, want %v", c.count, c.last, got, c.want)
			}
		})
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
