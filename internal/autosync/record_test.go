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
	s.Record(OutcomeSynced, 0)

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
	NewFileRecordStore(path, fixedWall("2026-06-08T10:00:00Z")).Record(OutcomeSynced, 0)
	NewFileRecordStore(path, fixedWall("2026-06-08T12:00:00Z")).Record(OutcomeOffline, 0)

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
	NewFileRecordStore(path, fixedWall("2026-06-08T10:00:00Z")).Record(OutcomeSynced, 0)
	NewFileRecordStore(path, fixedWall("2026-06-08T13:00:00Z")).Record(OutcomeError, 0)

	rec, _ := LoadRecord(path)
	if rec.LastOutcome != "error" {
		t.Fatalf("last_outcome = %q, want error", rec.LastOutcome)
	}
	if rec.LastSuccessAt == nil || !rec.LastSuccessAt.Equal(mustTime("2026-06-08T10:00:00Z")) {
		t.Fatalf("error must preserve the earlier success stamp, got %+v", rec.LastSuccessAt)
	}
}

func TestFileRecordStore_SuccessStoresCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-sync")
	s := NewFileRecordStore(path, fixedWall("2026-06-08T16:42:00Z"))
	s.Record(OutcomeSynced, 7)

	if got := s.LastCount(); got != 7 {
		t.Fatalf("LastCount() = %d, want 7", got)
	}
	rec, _ := LoadRecord(path)
	if rec.LastCount != 7 {
		t.Fatalf("persisted last_count = %d, want 7", rec.LastCount)
	}
}

func TestFileRecordStore_OfflinePreservesCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-sync")
	NewFileRecordStore(path, fixedWall("2026-06-08T10:00:00Z")).Record(OutcomeSynced, 9)
	NewFileRecordStore(path, fixedWall("2026-06-08T12:00:00Z")).Record(OutcomeOffline, 0)

	rec, _ := LoadRecord(path)
	if rec.LastCount != 9 {
		t.Fatalf("offline must preserve the prior count, got %d want 9", rec.LastCount)
	}
}

func TestFileRecordStore_ErrorPreservesCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-sync")
	NewFileRecordStore(path, fixedWall("2026-06-08T10:00:00Z")).Record(OutcomeSynced, 9)
	NewFileRecordStore(path, fixedWall("2026-06-08T13:00:00Z")).Record(OutcomeError, 0)

	rec, _ := LoadRecord(path)
	if rec.LastCount != 9 {
		t.Fatalf("error must preserve the prior count, got %d want 9", rec.LastCount)
	}
}

func TestFileRecordStore_LastCountMissingIsZero(t *testing.T) {
	s := NewFileRecordStore(filepath.Join(t.TempDir(), "nope"), fixedWall("2026-06-08T10:00:00Z"))
	if got := s.LastCount(); got != 0 {
		t.Fatalf("LastCount() on a missing record = %d, want 0", got)
	}
}

func mustTime(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }
