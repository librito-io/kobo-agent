package autosync

import (
	"errors"
	"os"
	"syscall"
)

// Locker provides a non-blocking single-instance lock. The flock impl is below;
// the orchestrator tests use a fake.
type Locker interface {
	// TryLock acquires the lock without blocking. (true, unlock, nil) on success;
	// (false, _, nil) when another process holds it (the dedup case); a non-nil
	// err is a real failure. unlock is always safe to call (no-op when !ok).
	TryLock() (ok bool, unlock func(), err error)
}

// flockLocker holds an exclusive non-blocking flock on path (production:
// /tmp/librito-autosync.lock on tmpfs — reliable flock, auto-release on death).
type flockLocker struct{ path string }

// NewFlockLocker builds a Locker on path.
func NewFlockLocker(path string) Locker { return &flockLocker{path: path} }

func (l *flockLocker) TryLock() (bool, func(), error) {
	noop := func() {}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, noop, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return false, noop, nil // held by another run — expected dedup, not an error
		}
		return false, noop, err
	}
	unlock := func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return true, unlock, nil
}
