package autosync

import (
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

// ResolveBaseURL returns the trimmed url-file content when the file was present
// and non-empty, else def. Pairing writes this file (Step 3 backend coupling),
// so in practice the resolved backend matches the one the token was minted on.
func ResolveBaseURL(content string, found bool, def string) string {
	if found {
		if u := strings.TrimSpace(content); u != "" {
			return u
		}
	}
	return def
}
