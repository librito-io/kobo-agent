package autosync

import "testing"

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
