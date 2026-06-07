package autosync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBaseURL(t *testing.T) {
	const def = "https://librito.io"
	cases := []struct {
		name    string
		content string
		found   bool
		want    string
	}{
		{"file absent → default", "", false, def},
		{"file empty → default", "", true, def},
		{"file whitespace → default", "  \n\t ", true, def},
		{"https passes", "https://librito.io", true, "https://librito.io"},
		{"https vercel prelaunch", "https://librito-web.vercel.app\n", true, "https://librito-web.vercel.app"},
		{"http to private LAN IP passes (trimmed)", "  http://192.168.68.54:5173\n", true, "http://192.168.68.54:5173"},
		{"http to loopback IP passes", "http://127.0.0.1:8080", true, "http://127.0.0.1:8080"},
		{"http to localhost passes", "http://localhost:5173", true, "http://localhost:5173"},
		// Plain http to a PUBLIC host would send the bearer token in cleartext →
		// refuse and fall back to the (https) default. Dev LAN/loopback http stays.
		{"http to public host → default", "http://librito.io", true, def},
		{"http to public host w/ path → default", "http://example.com/api", true, def},
		{"non-http(s) scheme → default", "ftp://192.168.0.2", true, def},
		{"malformed → default", "not a url", true, def},
		{"scheme only, no host → default", "https://", true, def},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveBaseURL(c.content, c.found, def); got != c.want {
				t.Fatalf("ResolveBaseURL(%q, %v) = %q, want %q", c.content, c.found, got, c.want)
			}
		})
	}
}

func TestFileConfig_Token(t *testing.T) {
	dir := t.TempDir()
	c := NewFileConfig(dir, "https://librito.io")

	if got := c.Token(); got != "" {
		t.Fatalf("Token() with no file = %q, want empty", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("sk_device_abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.Token(); got != "sk_device_abc" {
		t.Fatalf("Token() = %q, want sk_device_abc", got)
	}
}

func TestFileConfig_BaseURL(t *testing.T) {
	dir := t.TempDir()
	c := NewFileConfig(dir, "https://librito.io")

	if got := c.BaseURL(); got != "https://librito.io" {
		t.Fatalf("BaseURL() with no file = %q, want compiled default", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "url"), []byte("http://127.0.0.1:5173\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "http://127.0.0.1:5173" {
		t.Fatalf("BaseURL() = %q, want http://127.0.0.1:5173", got)
	}
}
