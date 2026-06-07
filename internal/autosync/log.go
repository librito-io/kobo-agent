package autosync

import (
	"bytes"
	"os"
	"time"
)

// Logger appends one autosync result line. The file impl is below; the
// orchestrator tests use an in-memory fake.
type Logger interface {
	Log(line string)
}

// FormatLine composes one log line: a UTC RFC3339 timestamp + "autosync: " +
// msg + newline. UTC is load-bearing — udev's RUN has no TZ and the Kobo
// wall-clock zone is unreliable (CLAUDE.md invariant #3), so the timestamp is
// always normalized to UTC regardless of t's location.
func FormatLine(t time.Time, msg string) string {
	return t.UTC().Format(time.RFC3339) + " autosync: " + msg + "\n"
}

// fileLogger appends lines to path and keeps the file under maxBytes by
// truncate-oldest. Production cap is ~64 KB so the log cannot grow unbounded on
// the FAT user partition.
type fileLogger struct {
	path     string
	maxBytes int
}

// NewFileLogger builds a Logger writing to path, capped at maxBytes.
func NewFileLogger(path string, maxBytes int) Logger {
	return &fileLogger{path: path, maxBytes: maxBytes}
}

func (l *fileLogger) Log(line string) {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return // logging is best-effort; a log failure never fails a sync
	}
	_, _ = f.WriteString(line)
	_ = f.Close()

	b, err := os.ReadFile(l.path)
	if err != nil || len(b) <= l.maxBytes {
		return
	}
	_ = os.WriteFile(l.path, capLog(b, l.maxBytes), 0o644)
}

// capLog returns content trimmed to at most max bytes, keeping the newest lines
// (truncate-oldest) and never starting mid-line. Content already within max is
// returned unchanged.
func capLog(content []byte, max int) []byte {
	if len(content) <= max {
		return content
	}
	tail := content[len(content)-max:]
	// Drop the (likely partial) leading line so the file starts on a boundary.
	if i := bytes.IndexByte(tail, '\n'); i >= 0 && i+1 <= len(tail) {
		tail = tail[i+1:]
	}
	return tail
}
