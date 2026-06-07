package autosync

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// buildSyncFixture writes a minimal fabricated KoboReader.sqlite (one visible
// highlight) so the real sync.Run adapter has something to POST. Fabricated data
// only — no personal reading data in the repo (CLAUDE.md Fixtures).
func buildSyncFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "KoboReader.sqlite")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, stmt := range []string{
		`CREATE TABLE content (ContentID TEXT PRIMARY KEY, ContentType TEXT, Title TEXT, Attribution TEXT, ISBN TEXT)`,
		`CREATE TABLE Bookmark (BookmarkID TEXT PRIMARY KEY, VolumeID TEXT, ContentID TEXT, Text TEXT, Annotation TEXT, DateCreated TEXT, Hidden TEXT, Type TEXT)`,
		`INSERT INTO content VALUES ('file:///a.epub','6','Book A','Auth A','9990000000017')`,
		`INSERT INTO Bookmark VALUES ('uid-1','file:///a.epub','file:///a.epub#c1','line one','','2026-06-04T19:30:02.858','false','highlight')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// The real adapter must pass the server's distinct {imported,books} straight
// through — a swap (or a return-value mapping regression) surfaces here. The
// orchestrator tests use a fake, so this is the only coverage of NewSyncer.
func TestSyncer_MapsImportedAndBooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/import/kobo" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"imported":3,"books":2}`)) // distinct so a swap is caught
	}))
	defer srv.Close()

	imported, books, err := NewSyncer(buildSyncFixture(t)).Sync(srv.URL, "sk_device_test")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if imported != 3 || books != 2 {
		t.Fatalf("Sync = (imported=%d, books=%d), want (3, 2)", imported, books)
	}
}

// A sync failure (here an unreadable DB path) must propagate with zero counts,
// not be swallowed — the orchestrator logs it and exits nonzero.
func TestSyncer_ErrorPropagates(t *testing.T) {
	imported, books, err := NewSyncer("/nonexistent/dir/KoboReader.sqlite").Sync("http://unused.invalid", "tok")
	if err == nil {
		t.Fatal("want error from a missing DB, got nil")
	}
	if imported != 0 || books != 0 {
		t.Fatalf("on error want (0, 0), got (%d, %d)", imported, books)
	}
}
