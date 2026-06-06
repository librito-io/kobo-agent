package pair

import (
	"testing"
	"time"
)

func TestMillis(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{20 * time.Second, "20000"},
		{1500 * time.Millisecond, "1500"},
		{0, "0"},
		{-5 * time.Second, "0"}, // clamped: never a huge unsigned wait
	}
	for _, c := range cases {
		if got := millis(c.in); got != c.want {
			t.Fatalf("millis(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
