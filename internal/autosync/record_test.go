package autosync

import (
	"path/filepath"
	"testing"
	"time"
)

func fixedWall(ts string) func() time.Time {
	t, _ := time.Parse(time.RFC3339, ts)
	return func() time.Time { return t }
}

func TestFileRecordStore_SuccessSetsBoth(t *testing.T) {
	dir := t.TempDir()
	s := NewFileRecordStore(filepath.Join(dir, "last-sync"), fixedWall("2026-06-08T16:42:00Z"))
	s.Record(OutcomeSynced)

	rec, ok := LoadRecord(filepath.Join(dir, "last-sync"))
	if !ok {
		t.Fatal("record not written")
	}
	if rec.LastOutcome != "ok" || rec.LastSuccessAt == nil || rec.LastAttemptAt == nil {
		t.Fatalf("synced should set ok + both stamps, got %+v", rec)
	}
}

func TestFileRecordStore_OfflinePreservesPriorSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-sync")
	NewFileRecordStore(path, fixedWall("2026-06-08T10:00:00Z")).Record(OutcomeSynced)
	NewFileRecordStore(path, fixedWall("2026-06-08T12:00:00Z")).Record(OutcomeOffline)

	rec, _ := LoadRecord(path)
	if rec.LastOutcome != "offline" {
		t.Fatalf("last_outcome = %q, want offline", rec.LastOutcome)
	}
	if rec.LastSuccessAt == nil || !rec.LastSuccessAt.Equal(mustTime("2026-06-08T10:00:00Z")) {
		t.Fatalf("offline must preserve the earlier success stamp, got %+v", rec.LastSuccessAt)
	}
}

func TestLoadRecord_MissingFile(t *testing.T) {
	if _, ok := LoadRecord(filepath.Join(t.TempDir(), "nope")); ok {
		t.Fatal("missing file should report ok=false")
	}
}

func TestFileRecordStore_ErrorPreservesPriorSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-sync")
	NewFileRecordStore(path, fixedWall("2026-06-08T10:00:00Z")).Record(OutcomeSynced)
	NewFileRecordStore(path, fixedWall("2026-06-08T13:00:00Z")).Record(OutcomeError)

	rec, _ := LoadRecord(path)
	if rec.LastOutcome != "error" {
		t.Fatalf("last_outcome = %q, want error", rec.LastOutcome)
	}
	if rec.LastSuccessAt == nil || !rec.LastSuccessAt.Equal(mustTime("2026-06-08T10:00:00Z")) {
		t.Fatalf("error must preserve the earlier success stamp, got %+v", rec.LastSuccessAt)
	}
}

func mustTime(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }
