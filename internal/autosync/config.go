package autosync

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config supplies the two paired-file reads the orchestrator depends on. The
// file impl is below; the orchestrator tests use a plain struct fake.
type Config interface {
	Token() string   // trimmed token-file contents; "" if absent/empty (the no-token guard)
	BaseURL() string // resolved url file → backend, else the compiled default
}

// fileConfig reads token + url from dir (production: /mnt/onboard/.adds/librito/).
type fileConfig struct {
	dir        string
	defaultURL string
}

// NewFileConfig builds a Config rooted at dir, falling back to defaultURL when
// no url file was written by pairing.
func NewFileConfig(dir, defaultURL string) Config {
	return &fileConfig{dir: dir, defaultURL: defaultURL}
}

func (c *fileConfig) Token() string {
	b, err := os.ReadFile(filepath.Join(c.dir, "token"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (c *fileConfig) BaseURL() string {
	b, err := os.ReadFile(filepath.Join(c.dir, "url"))
	return ResolveBaseURL(string(b), err == nil, c.defaultURL)
}

// ResolveBaseURL returns the validated url-file backend, else def. Pairing writes
// this file (Step 3 backend coupling), so in practice the resolved backend matches
// the one the token was minted on. The value is validated rather than trusted
// verbatim: it must be an absolute http(s) URL, and plain http is honored only for
// loopback/private hosts (the dev-LAN path) — sending the bearer token in cleartext
// to a public host is refused. Anything missing/empty/malformed/unsafe → def, so a
// bad url file fails safe to the compiled default instead of at POST time.
func ResolveBaseURL(content string, found bool, def string) string {
	if !found {
		return def
	}
	raw := strings.TrimSpace(content)
	if raw == "" {
		return def
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return def
	}
	switch u.Scheme {
	case "https":
		return raw
	case "http":
		if isLoopbackOrPrivate(u.Hostname()) {
			return raw
		}
	}
	return def
}

// isLoopbackOrPrivate reports whether host is localhost or a loopback/RFC1918 IP —
// the only hosts allowed to receive the bearer token over plain http.
func isLoopbackOrPrivate(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}
