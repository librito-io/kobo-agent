package autosync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// record.go implements the last-sync record: a small JSON file read-modify-written
// by RecordStore after each sync attempt, read back only by `agent status`.

// --- TEMPORARY STUB: remove when Task 2 lands the full Outcome enum ---
type Outcome int

const (
	OutcomeSynced  Outcome = iota
	OutcomeOffline         // will be wired to connectivity-timeout path
	OutcomeError           // will be wired to sync-error path
)

// --- END TEMPORARY STUB ---

// Record is the persisted last-sync state. Its ONLY reader is `agent status`
// (via internal/status.DecideStatusLine); sync-now uses Run's return value, not
// this file. Timestamps are wall-clock (calendar) time — see the spec's
// "Time on the Kobo" section; trustworthy here because the write happens right
// after a successful networked TLS POST (clock is NTP-set + cert-valid-range).
type Record struct {
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"` // last OK sync; nil = never
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"` // last attempt of any kind
	LastOutcome   string     `json:"last_outcome,omitempty"`    // "ok" | "offline" | "error"
}

// RecordStore writes the last-sync record. The file impl is below; Run holds the
// interface so tests use a fake. Dedup/Unpaired/LockErr never call it.
type RecordStore interface {
	Record(o Outcome)
}

type fileRecordStore struct {
	path string
	now  func() time.Time // injected WALL clock (never the monotonic autosync Clock)
}

// NewFileRecordStore builds a RecordStore at path, stamping with now (a wall clock).
func NewFileRecordStore(path string, now func() time.Time) RecordStore {
	return &fileRecordStore{path: path, now: now}
}

// Record read-modify-writes the file so a non-success attempt preserves the prior
// last_success_at (the durable "it worked once" truth the status line leans on).
func (s *fileRecordStore) Record(o Outcome) {
	rec, _ := LoadRecord(s.path) // missing → zero Record
	t := s.now().UTC()
	rec.LastAttemptAt = &t
	switch o {
	case OutcomeSynced:
		rec.LastSuccessAt = &t
		rec.LastOutcome = "ok"
	case OutcomeOffline:
		rec.LastOutcome = "offline"
	case OutcomeError:
		rec.LastOutcome = "error"
	default:
		// Intentionally silent + no write: Dedup/Unpaired/LockErr are not sync
		// attempts, so they must not touch the record (not even LastAttemptAt).
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return // best-effort; a record failure never fails a sync
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	_ = os.WriteFile(s.path, b, 0o600)
}

// LoadRecord reads the record file. ok=false when absent or unparseable (the
// caller treats that as "never synced").
func LoadRecord(path string) (Record, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Record{}, false
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return Record{}, false
	}
	return rec, true
}
