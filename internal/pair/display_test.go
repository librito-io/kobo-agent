package pair

import "testing"

func TestParseButton(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Button
	}{
		// qndb prints the signal name then the int result on one line.
		{"reject → cancel", "dlgConfirmResult 0", ButtonCancel},
		{"accept → retry", "dlgConfirmResult 1", ButtonRetry},
		{"bare 0", "0", ButtonCancel},
		{"bare 1", "1", ButtonRetry},
		{"trailing whitespace", "dlgConfirmResult 1\n", ButtonRetry},
		{"empty → none", "", ButtonNone},
		{"unknown value → none", "dlgConfirmResult 2", ButtonNone},
		{"non-numeric → none", "dlgConfirmResult x", ButtonNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseButton(c.in); got != c.want {
				t.Fatalf("parseButton(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestLastField(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"dlgConfirmResult 1", "1"},
		{"single", "single"},
		{"  spaced   tokens  ", "tokens"},
		{"", ""},
		{"\n\t", ""},
	}
	for _, c := range cases {
		if got := lastField(c.in); got != c.want {
			t.Fatalf("lastField(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSpaceOut(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"482913", "4 8 2 9 1 3"},
		{"1", "1"},
		{"", ""},
	}
	for _, c := range cases {
		if got := spaceOut(c.in); got != c.want {
			t.Fatalf("spaceOut(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
