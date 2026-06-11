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

func TestGrowth(t *testing.T) {
	cases := []struct {
		name        string
		count, last int
		want        int
	}{
		{"first sync from zero", 1, 0, 1},
		{"nothing ever, nothing now", 0, 0, 0},
		{"offline backlog flushed", 13, 10, 3},
		{"resend of unchanged set", 13, 13, 0},
		{"single new highlight", 6, 5, 1},
		{"device-side delete shrinks set", 5, 10, -5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Growth(c.count, c.last); got != c.want {
				t.Errorf("Growth(%d, %d) = %d, want %d", c.count, c.last, got, c.want)
			}
		})
	}
}

func TestToastText(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "1 new highlight synced to Librito"},
		{2, "2 new highlights synced to Librito"},
		{13, "13 new highlights synced to Librito"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := ToastText(c.n); got != c.want {
				t.Errorf("ToastText(%d) = %q, want %q", c.n, got, c.want)
			}
		})
	}
}

func TestDefaultToastAllow_HomeAndLibraryOnly(t *testing.T) {
	want := []string{"HomePageView", "DragonLibraryView"}
	if len(defaultToastAllow) != len(want) {
		t.Fatalf("defaultToastAllow = %v, want %v", defaultToastAllow, want)
	}
	for i, v := range want {
		if defaultToastAllow[i] != v {
			t.Fatalf("defaultToastAllow = %v, want %v", defaultToastAllow, want)
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
