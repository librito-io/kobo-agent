//go:build linux

package watch

import (
	"bytes"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// inotifyWatcher watches a single directory. We watch the DIRECTORY (stable
// inode), not the WAL file, because the WAL is deleted/recreated on checkpoint —
// a directory watch survives that with no re-arm.
type inotifyWatcher struct {
	fd        int
	wd        int
	events    chan Event
	done      chan struct{}
	closeOnce sync.Once
}

// NewWatcher arms inotify on dir for create/modify/move-in events and starts a
// goroutine that surfaces them on Events().
func NewWatcher(dir string) (Watcher, error) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("inotify init: %w", err)
	}
	wd, err := unix.InotifyAddWatch(fd, dir, unix.IN_CREATE|unix.IN_MODIFY|unix.IN_MOVED_TO)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("inotify add watch %s: %w", dir, err)
	}
	w := &inotifyWatcher{fd: fd, wd: wd, events: make(chan Event, 64), done: make(chan struct{})}
	go w.loop()
	return w, nil
}

func (w *inotifyWatcher) Events() <-chan Event { return w.events }

func (w *inotifyWatcher) Close() error {
	// Idempotent: a second Close() must not panic on close(w.done). The Watcher
	// interface states no call-once contract.
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		_, _ = unix.InotifyRmWatch(w.fd, uint32(w.wd))
		err = unix.Close(w.fd) // unblocks a blocked Read with EBADF → loop exits
	})
	return err
}

func (w *inotifyWatcher) loop() {
	defer close(w.events)
	var buf [4096]byte
	for {
		n, err := unix.Read(w.fd, buf[:])
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return // fd closed (Close) or fatal
		}
		var offset int
		for offset+unix.SizeofInotifyEvent <= n {
			raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			name := ""
			if nameLen := int(raw.Len); nameLen > 0 {
				start := offset + unix.SizeofInotifyEvent
				// Defensive: the kernel only returns complete records that fit n,
				// so this never trips in practice — but bound the slice to the bytes
				// actually read so a malformed Len can't read stale buffer residue.
				if start+nameLen > n {
					break
				}
				name = string(bytes.TrimRight(buf[start:start+nameLen], "\x00"))
			}
			select {
			case w.events <- Event{Name: name, Mask: raw.Mask}:
			case <-w.done:
				return
			}
			offset += unix.SizeofInotifyEvent + int(raw.Len)
		}
	}
}
