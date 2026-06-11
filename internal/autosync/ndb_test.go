package autosync

import (
	"path/filepath"
	"testing"
)

// TestQndbBinIsAbsolute guards the udev-no-PATH regression: the WiFi-up autosync
// is launched by udev, whose RUN children inherit NO PATH, so a bare command name
// fails exec.LookPath and the view probe + toast silently no-op (sync still runs).
// qndb must be invoked by absolute path.
func TestQndbBinIsAbsolute(t *testing.T) {
	if !filepath.IsAbs(qndbBin) {
		t.Fatalf("qndbBin = %q, must be an absolute path (udev RUN children have no PATH)", qndbBin)
	}
}

// TestClampToastMs guards the NDB hard ceiling: NDB 0.2.0 asserts
// 0 < toastDuration <= 5000 (NDBDbus.cc mwcToast) and REJECTS the call outside
// that range — an over-limit duration doesn't truncate, it silently loses the
// toast. Clamp so a future "make it longer" edit can't kill toasts entirely.
func TestClampToastMs(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"negative", -5, 1},
		{"zero (NDB rejects 0)", 0, 1},
		{"normal", 2000, 2000},
		{"at ceiling", 5000, 5000},
		{"over ceiling", 6000, 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampToastMs(c.in); got != c.want {
				t.Errorf("clampToastMs(%d) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestLastLineField(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "  \n\t ", ""},
		{"single word", "home", "home"},
		{"signal prefix then value", "ndbCurrentView home", "home"},
		{"multi-line, value on last line", "ok\nndbCurrentView ReadingView", "ReadingView"},
		{"trailing newline", "home\n", "home"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastLineField(c.in); got != c.want {
				t.Errorf("lastLineField(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
