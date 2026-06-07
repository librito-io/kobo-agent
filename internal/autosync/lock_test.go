package autosync

import (
	"path/filepath"
	"testing"
)

func TestFlockLocker_AcquireBlockRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autosync.lock")
	l := NewFlockLocker(path)

	ok, unlock, err := l.TryLock()
	if err != nil {
		t.Fatalf("first TryLock: %v", err)
	}
	if !ok {
		t.Fatal("first TryLock should acquire the lock")
	}

	// A second locker on the SAME path must fail to acquire while held.
	ok2, _, err2 := NewFlockLocker(path).TryLock()
	if err2 != nil {
		t.Fatalf("second TryLock errored: %v", err2)
	}
	if ok2 {
		t.Fatal("second TryLock should NOT acquire while the first holds the lock")
	}

	// Release the first → a third acquire succeeds.
	unlock()
	ok3, unlock3, err3 := NewFlockLocker(path).TryLock()
	if err3 != nil {
		t.Fatalf("third TryLock: %v", err3)
	}
	if !ok3 {
		t.Fatal("third TryLock should acquire after release")
	}
	unlock3()
}
