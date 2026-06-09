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
