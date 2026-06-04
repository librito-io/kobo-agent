package sync

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// buildRunFixture writes a minimal fabricated KoboReader.sqlite for the Run
// orchestrator test: two visible highlights, one with a real (fabricated) ISBN
// and one with a calibre:N junk ISBN that must map to a nil ISBN downstream.
func buildRunFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "KoboReader.sqlite")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE content (ContentID TEXT PRIMARY KEY, ContentType TEXT, Title TEXT, Attribution TEXT, ISBN TEXT)`,
		`CREATE TABLE Bookmark (BookmarkID TEXT PRIMARY KEY, VolumeID TEXT, ContentID TEXT, Text TEXT, Annotation TEXT, DateCreated TEXT, Hidden TEXT, Type TEXT)`,
		`INSERT INTO content VALUES ('file:///a.epub','6','Book A','Auth A','9990000000017')`,
		`INSERT INTO content VALUES ('file:///b.epub','6','Book B','Auth B','calibre:4')`,
		`INSERT INTO Bookmark VALUES ('uid-real','file:///a.epub','file:///a.epub#c1','line one','','2026-06-04T19:30:02.858','false','highlight')`,
		`INSERT INTO Bookmark VALUES ('uid-junk','file:///b.epub','file:///b.epub#c1','line two','','2026-06-04T19:41:53.758','false','highlight')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// Run wires read → map → (post|dry-run). Dry-run must build both items, null
// the junk calibre ISBN, and make no HTTP call.
func TestRun_DryRunBuildsItemsNoPost(t *testing.T) {
	out, err := Run(Options{
		DBPath:  buildRunFixture(t),
		BaseURL: "http://unused.invalid", // must NOT be contacted in dry-run
		Token:   "tok",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	if out.Built != 2 {
		t.Fatalf("built %d items, want 2", out.Built)
	}
	if out.Posted {
		t.Fatal("dry-run must not post")
	}

	var foundJunk bool
	for _, it := range out.Items {
		if it.SourceUID == "uid-junk" {
			foundJunk = true
			if it.ISBN != nil {
				t.Fatalf("junk-ISBN item ISBN = %q, want nil", *it.ISBN)
			}
		}
	}
	if !foundJunk {
		t.Fatal("expected the calibre-isbn highlight in built items")
	}
}
