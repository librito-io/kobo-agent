//go:build !linux

package watch

import "errors"

// NewWatcher is unsupported off Linux (inotify is Linux-only). The stub lets the
// package compile on the darwin dev machine; `agent watch` is only ever run on
// the Kobo (Linux/arm).
func NewWatcher(dir string) (Watcher, error) {
	return nil, errors.New("watch: inotify is only supported on linux")
}
