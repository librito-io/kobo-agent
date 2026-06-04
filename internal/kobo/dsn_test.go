package kobo

import (
	"os"
	"path/filepath"
	"testing"
)

// The --db path is interpolated into a SQLite file: URI DSN. Special URI chars
// in the path must be escaped, not interpreted: a '#' would truncate the path
// as a URI fragment (open the wrong/no file), and a '?'/'&' would inject DSN
// query params — an injected mode=rwc could override the appended mode=ro and
// defeat the read-only invariant. These build a real DB at an awkward path and
// assert ReadHighlights still opens exactly that file, read-only.
func TestReadHighlights_PathWithHash(t *testing.T) {
	// Directory name contains '#', a URI fragment delimiter.
	dir := filepath.Join(t.TempDir(), "lib#1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "KoboReader.sqlite")
	writeFixtureDBAt(t, path, defaultFixtureRows())

	rows, err := ReadHighlights(path)
	if err != nil {
		t.Fatalf("ReadHighlights on '#' path: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d rows from '#' path, want 7 (path likely truncated at #)", len(rows))
	}
}

func TestReadHighlights_PathWithQueryChars(t *testing.T) {
	// A '?' in the path would otherwise start a DSN query string; a literal
	// 'mode=rwc' in the path must NOT become an actual DSN param.
	dir := filepath.Join(t.TempDir(), "q?mode=rwc&x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "KoboReader.sqlite")
	writeFixtureDBAt(t, path, defaultFixtureRows())

	rows, err := ReadHighlights(path)
	if err != nil {
		t.Fatalf("ReadHighlights on '?'-laden path: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7", len(rows))
	}
}
