package watch

import (
	"errors"
	"testing"
	"time"

	"github.com/librito-io/kobo-sync/internal/autosync"
)

// Minimal fakes for the autosync edges the two non-network exit paths touch.
type rtLocker struct {
	ok  bool
	err error
}

func (l rtLocker) TryLock() (bool, func(), error) {
	if l.err != nil {
		return false, func() {}, l.err
	}
	return l.ok, func() {}, nil
}

type rtConfig struct{ token string }

func (c rtConfig) Token() string   { return c.token }
func (c rtConfig) BaseURL() string { return "" }

type rtLogger struct{}

func (rtLogger) Log(string) {}

type rtClock struct{}

func (rtClock) Now() time.Time        { return time.Unix(0, 0) }
func (rtClock) Sleep(d time.Duration) {}

func TestAutosyncRunner_NonzeroExit_Errors(t *testing.T) {
	// A lock error makes autosync.Run return a nonzero exit code.
	deps := autosync.Deps{
		Locker: rtLocker{err: errors.New("flock: permission denied")},
		Logger: rtLogger{},
		Clock:  rtClock{},
	}
	if err := NewRunner(deps).Sync(); err == nil {
		t.Fatal("Sync() = nil, want error on a nonzero autosync exit")
	}
}

func TestAutosyncRunner_ZeroExit_Nil(t *testing.T) {
	// Lock acquired + empty token → autosync.Run's unpaired no-op returns exit 0.
	deps := autosync.Deps{
		Locker: rtLocker{ok: true},
		Config: rtConfig{token: ""},
		Logger: rtLogger{},
		Clock:  rtClock{},
	}
	if err := NewRunner(deps).Sync(); err != nil {
		t.Fatalf("Sync() = %v, want nil on exit-0 (unpaired no-op)", err)
	}
}
