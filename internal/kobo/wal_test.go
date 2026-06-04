package kobo

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func statSize(p string) (int64, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Guards the load-bearing WAL property: Nickel keeps the highlight DB in WAL
// mode and writes new highlights to the -wal sidecar, checkpointing on its own
// schedule. An agent that read only the main .sqlite file would silently miss
// every highlight made since the last checkpoint. This builds a DB whose only
// highlight lives in an UN-checkpointed WAL and asserts ReadHighlights sees it.
func TestReadHighlights_SeesUncheckpointedWAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KoboReader.sqlite")

	// Writer connection: WAL mode, autocheckpoint disabled so the insert stays
	// in the -wal file and never folds into the main db.
	w, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=wal_autocheckpoint(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	schema := []string{
		`CREATE TABLE content (ContentID TEXT PRIMARY KEY, ContentType TEXT, Title TEXT, Attribution TEXT, ISBN TEXT)`,
		`CREATE TABLE Bookmark (BookmarkID TEXT PRIMARY KEY, VolumeID TEXT, ContentID TEXT, Text TEXT, Annotation TEXT, DateCreated TEXT, Hidden TEXT, Type TEXT)`,
	}
	for _, s := range schema {
		if _, err := w.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Exec(
		`INSERT INTO content VALUES ('file:///b.epub','6','WAL Book','WAL Author','9780593723746')`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Exec(
		`INSERT INTO Bookmark VALUES ('wal-uid','file:///b.epub','file:///b.epub#ch1','only in wal','', '2026-06-04T12:00:00.000','false','highlight')`,
	); err != nil {
		t.Fatal(err)
	}

	// Confirm the precondition: a non-trivial WAL exists (row not checkpointed).
	walPath := path + "-wal"
	if fi, err := statSize(walPath); err != nil || fi == 0 {
		t.Fatalf("expected a populated -wal sidecar (size>0), got size=%d err=%v", fi, err)
	}

	// The agent's reader (separate connection) must still see the WAL row.
	rows, err := ReadHighlights(path)
	if err != nil {
		t.Fatalf("ReadHighlights: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d highlights, want 1 (the WAL-only row)", len(rows))
	}
	if rows[0].BookmarkID != "wal-uid" || rows[0].Text != "only in wal" {
		t.Fatalf("wrong row: %+v", rows[0])
	}
}
