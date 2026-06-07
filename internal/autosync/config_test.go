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
		{"valid url", "https://librito.io", true, "https://librito.io"},
		{"valid url trimmed", "  http://192.168.68.54:5173\n", true, "http://192.168.68.54:5173"},
		{"vercel prelaunch url", "https://librito-web.vercel.app\n", true, "https://librito-web.vercel.app"},
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
	if err := os.WriteFile(filepath.Join(dir, "url"), []byte("http://dev:5173\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL(); got != "http://dev:5173" {
		t.Fatalf("BaseURL() = %q, want http://dev:5173", got)
	}
}
