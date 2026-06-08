package pair

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// fileStore persists hardware-id + token under dir (production:
// /mnt/onboard/.adds/librito/). Files are 0600; dir is created if missing.
// .adds/ is USB-exported, so the token is world-readable to anyone who mounts
// the device — the real protection is the SSH-hardening prereq (spec §Prereq).
type fileStore struct {
	dir string
	rnd io.Reader // random source for hardware-id generation (crypto/rand in prod)
}

// NewFileStore builds a Store rooted at dir, generating IDs from rnd.
func NewFileStore(dir string, rnd io.Reader) Store {
	return &fileStore{dir: dir, rnd: rnd}
}

func (s *fileStore) LoadOrCreateHardwareID() (string, error) {
	path := filepath.Join(s.dir, "hardware-id")
	if b, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(b))
		if ValidateHardwareID(id) {
			return id, nil // reuse byte-for-byte
		}
		// Corrupt/invalid (e.g. hand-edited, uppercase) → regenerate rather than
		// risk a phantom device on a mixed-case id.
	}
	id, err := GenerateHardwareID(s.rnd)
	if err != nil {
		return "", err
	}
	if err := s.write(path, id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *fileStore) WriteToken(token string) error {
	return s.write(filepath.Join(s.dir, "token"), token)
}

func (s *fileStore) WriteURL(url string) error {
	return s.write(filepath.Join(s.dir, "url"), url)
}

func (s *fileStore) WriteAccount(email string, pairedAt time.Time) error {
	if err := s.write(filepath.Join(s.dir, "email"), email); err != nil {
		return err
	}
	return s.write(filepath.Join(s.dir, "paired-at"), pairedAt.UTC().Format(time.RFC3339))
}

func (s *fileStore) write(path, content string) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content+"\n"), 0o600)
}
