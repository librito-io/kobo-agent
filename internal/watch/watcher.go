// Package watch implements the `watch` subcommand: a resident daemon that
// detects new highlights via inotify on the KoboReader.sqlite directory and,
// only when already connected, delegates an immediate sync to autosync.Run.
//
// The resident loop (watch.go) is pure over its interfaces — Watcher, SigReader,
// Prober, Runner, Locker (+ Clock, Logger) — so its decisions are table-tested on
// fakes. The inotify edge (inotify_linux.go) is hardware-verified. inotify is
// Linux-only; NewWatcher has a !linux stub so the package still builds on darwin.
package watch

// Event is one inotify event in the watched directory. Name is the basename of
// the file the event concerns (e.g. "KoboReader.sqlite-wal"); Mask is the raw
// inotify mask (surfaced for --probe diagnostics).
type Event struct {
	Name string
	Mask uint32
}

// Watcher surfaces filesystem events for the watched directory on a channel. The
// inotify impl is in inotify_linux.go; the loop tests use a fake feeding scripted
// events. Events() is closed when the watcher stops (the loop treats a closed
// channel as shutdown).
type Watcher interface {
	Events() <-chan Event
	Close() error
}
