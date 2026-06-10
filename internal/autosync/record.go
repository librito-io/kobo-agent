package autosync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// record.go implements the last-sync record: a small JSON file read-modify-written
// by RecordStore after each sync attempt, read back only by `kobo-sync status`.

// Record is the persisted last-sync state. Its ONLY reader is `kobo-sync status`
// (via internal/status.DecideStatusLine); sync-now uses Run's return value, not
// this file. Timestamps are wall-clock (calendar) time — see the spec's
// "Time on the Kobo" section; trustworthy here because the write happens right
// after a successful networked TLS POST (clock is NTP-set + cert-valid-range).
type Record struct {
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"` // last OK sync; nil = never
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"` // last attempt of any kind
	LastOutcome   string     `json:"last_outcome,omitempty"`    // "ok" | "offline" | "error"
	LastCount     int        `json:"last_count,omitempty"`      // highlight count at last OK sync (INTERNAL — the wake-toast growth gate; never displayed)
}

// RecordStore writes the last-sync record and reads back the last highlight count.
// The file impl is below; Run holds the interface so tests use a fake.
// Dedup/Unpaired/LockErr never call Record. count is persisted only on success;
// non-success outcomes preserve the prior count (it's still the last KNOWN total).
type RecordStore interface {
	Record(o Outcome, count int)
	LastCount() int
}

type fileRecordStore struct {
	path string
	now  func() time.Time // injected WALL clock (never the monotonic autosync Clock)
}

// NewFileRecordStore builds a RecordStore at path, stamping with now (a wall clock).
func NewFileRecordStore(path string, now func() time.Time) RecordStore {
	return &fileRecordStore{path: path, now: now}
}

// LastCount returns the highlight count recorded at the last successful sync (0
// when the record is absent/unparseable). It's read BEFORE Record overwrites it,
// to gate the wake toast on growth.
func (s *fileRecordStore) LastCount() int {
	rec, _ := LoadRecord(s.path)
	return rec.LastCount
}

// Record read-modify-writes the file so a non-success attempt preserves the prior
// last_success_at (the durable "it worked once" truth the status line leans on)
// and last_count (the growth-gate baseline; only a success moves it).
func (s *fileRecordStore) Record(o Outcome, count int) {
	rec, _ := LoadRecord(s.path) // missing → zero Record
	t := s.now().UTC()
	rec.LastAttemptAt = &t
	switch o {
	case OutcomeSynced:
		rec.LastSuccessAt = &t
		rec.LastOutcome = "ok"
		rec.LastCount = count // persist only on success; non-success keeps the loaded prior count
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
	// Write to a temp file then rename: rename is atomic on the same filesystem,
	// so `kobo-sync status` (which reads WITHOUT the shared sync lock) can never observe
	// a torn/half-written record. Writer-vs-writer is already serialized by the lock.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
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
