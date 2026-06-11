package kobo

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// fixtureRow is one fabricated highlight + its book/chapter metadata. All
// values are FICTIONAL — no real titles, authors, ISBNs, or highlight text.
// What IS preserved is the real Kobo data *shape* the reader must handle:
// the epub#fragment chapter ContentID, the calibre:N junk-ISBN string, a
// leading-tab in Text, and a separate ContentType=9 chapter row supplying the
// chapter title via the LEFT JOIN.
type fixtureRow struct {
	bookmarkID  string
	volumeID    string // book root (== content.ContentID for the ContentType=6 row)
	contentID   string // chapter fragment (volumeID + "#" + frag)
	text        string
	dateCreated string
	hidden      string // "true" | "false"
	bookTitle   string
	author      string
	isbn        string
	chapterTtl  string // chapter content.Title (ContentType=9)
	typ         string // Bookmark.Type; "" → 'highlight'. Real lifecycle: noting a
	// highlight flips the SAME row to 'note' (Text kept, Annotation filled), and a
	// note created straight from a selection is born 'note' — so 'note' rows are
	// highlights-with-notes and MUST be read. 'dogear' is a page mark (no Text).
	annotation string // Bookmark.Annotation; the note text on a 'note' row
}

// defaultFixtureRows is the canonical set the read tests assert against. Six
// visible highlights + one Hidden row that must be filtered out.
func defaultFixtureRows() []fixtureRow {
	return []fixtureRow{
		// Real-ISBN book (fabricated 13-digit ISBN) — ISBN-first resolve path.
		{"bk-realisbn", "file:///mnt/onboard/Books/Glass Tide.epub", "file:///mnt/onboard/Books/Glass Tide.epub#(2)ch01.xhtml",
			"The harbor froze over before anyone thought to leave.", "2026-06-04T19:30:02.858", "false",
			"Glass Tide", "Mara Quill", "9990000000017", "Chapter 1: Salt", "", ""},
		// calibre:N junk ISBN — the read layer passes it through raw; BuildItem
		// nulls it downstream → synthesis path. The literal "calibre:4" is a
		// Calibre format string, not personal data, so it stays verbatim.
		{"bk-calibre", "file:///mnt/onboard/Books/Field Notes.epub", "file:///mnt/onboard/Books/Field Notes.epub#(5)ch01.xhtml",
			"A list is a way of refusing to forget.", "2026-06-04T19:41:53.758", "false",
			"Field Notes", "Dov Ashir", "calibre:4", "On Lists", "", ""},
		// Chapter-title join (ContentType=9 row) with a richly-named chapter.
		{"bk-chaptertitle", "file:///mnt/onboard/Books/The Other Room.epub", "file:///mnt/onboard/Books/The Other Room.epub#(3)ch02.xhtml",
			"She had never noticed the second door.", "2026-06-04T19:42:35.372", "false",
			"The Other Room", "Pell Vance", "9990000000024", "Chapter 1: The Other Door", "", ""},
		// Leading-tab + curly-apostrophe text — CleanText trim + unicode preserve.
		{"bk-leadingtab", "file:///mnt/onboard/Books/Half Light.epub", "file:///mnt/onboard/Books/Half Light.epub#(1)ch01.xhtml",
			"\t I’m not sure the morning ever fully arrived.", "2026-06-02T19:36:33.000", "false",
			"Half Light", "Juno Sefer", "9990000000031", "Chapter 1", "", ""},
		// Whole-second timestamp (no millis) — another date shape.
		{"bk-wholesec", "file:///mnt/onboard/Books/Preludes.epub", "file:///mnt/onboard/Books/Preludes.epub#(4)ch00.xhtml",
			"Every preface is an apology in disguise.", "2026-06-04T19:30:38.944", "false",
			"Preludes", "Cato Reyes", "9990000000048", "Preface", "", ""},
		// Author in Last,First shape — read layer carries it raw.
		{"bk-lastfirst", "file:///mnt/onboard/Books/Night Ferry.epub", "file:///mnt/onboard/Books/Night Ferry.epub#(2)ch01.xhtml",
			"The others lined up along the rail, squinting.", "2026-06-04T19:32:39.811", "false",
			"Night Ferry", "Vance,Pell", "9990000000055", "Chapter One", "", ""},
		// Genuinely ISBN-less sideload (empty content.ISBN, NOT a junk string) —
		// a DRM-free epub with no identifier metadata. Distinct from the
		// calibre:N case: read layer carries "" and BuildItem also routes it to
		// the content_id/book_hash synthesis path. This is the true no-ISBN
		// case the live device data never produced.
		{"bk-noisbn", "file:///mnt/onboard/Books/Unsigned.epub", "file:///mnt/onboard/Books/Unsigned.epub#(1)ch01.xhtml",
			"No spine, no number, no claim on it but mine.", "2026-06-04T19:55:10.120", "false",
			"Unsigned", "Wren Halloway", "", "Chapter 1", "", ""},
		// Hidden (device-deleted) row — MUST be excluded by the read query.
		{"bk-hidden", "file:///mnt/onboard/Books/Glass Tide.epub", "file:///mnt/onboard/Books/Glass Tide.epub#(9)ch08.xhtml",
			"This one was deleted on the device.", "2026-06-04T19:50:00.000", "true",
			"Glass Tide", "Mara Quill", "9990000000017", "Chapter 8", "", ""},
		// Noted highlight: Nickel flips Type to 'note' when a note is added (and
		// births direct notes as 'note') — Text is still the highlighted passage
		// and MUST be read (#41). Latest DateCreated on purpose: the signature's
		// max must include 'note' rows too.
		{"bk-noted", "file:///mnt/onboard/Books/Glass Tide.epub", "file:///mnt/onboard/Books/Glass Tide.epub#(4)ch03.xhtml",
			"The lighthouse keeper kept two logs: one true.", "2026-06-04T20:01:02.500", "false",
			"Glass Tide", "Mara Quill", "9990000000017", "Chapter 3: Twin Logs", "note", "Which one did she burn?"},
		// Dogear (page mark): no Text on the real device — never synced.
		{"bk-dogear", "file:///mnt/onboard/Books/Glass Tide.epub", "file:///mnt/onboard/Books/Glass Tide.epub#(6)ch05.xhtml",
			"", "2026-06-04T20:05:00.000", "false",
			"Glass Tide", "Mara Quill", "9990000000017", "Chapter 5", "dogear", ""},
	}
}

// writeFixtureDB builds a KoboReader.sqlite in a temp dir from the given rows
// and returns its path. The schema mirrors the real device's Bookmark/content
// tables for the columns the reader touches. Distinct books get a ContentType=6
// book row (keyed by VolumeID); each highlight gets a ContentType=9 chapter row
// (keyed by the chapter-fragment ContentID) carrying the chapter title.
func writeFixtureDB(t *testing.T, rows []fixtureRow) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "KoboReader.sqlite")
	writeFixtureDBAt(t, path, rows)
	return path
}

// writeFixtureDBAt builds the fixture at an explicit path (for tests that need
// an awkward path, e.g. one containing URI-special characters). The DSN here
// uses url.PathEscape so the helper itself isn't subject to the bug under test.
func writeFixtureDBAt(t *testing.T, path string, rows []fixtureRow) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+url.PathEscape(path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, stmt := range []string{
		`CREATE TABLE content (ContentID TEXT PRIMARY KEY, ContentType TEXT, Title TEXT, Attribution TEXT, ISBN TEXT)`,
		`CREATE TABLE Bookmark (BookmarkID TEXT PRIMARY KEY, VolumeID TEXT, ContentID TEXT, Text TEXT, Annotation TEXT, DateCreated TEXT, Hidden TEXT, Type TEXT)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	seenBook := map[string]bool{}
	for _, r := range rows {
		if !seenBook[r.volumeID] {
			if _, err := db.Exec(
				`INSERT INTO content (ContentID, ContentType, Title, Attribution, ISBN) VALUES (?, '6', ?, ?, ?)`,
				r.volumeID, r.bookTitle, r.author, r.isbn,
			); err != nil {
				t.Fatal(err)
			}
			seenBook[r.volumeID] = true
		}
		// Chapter content row (ContentType=9) — title only; ISBN/author empty,
		// matching the real device where chapter rows carry no ISBN.
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO content (ContentID, ContentType, Title, Attribution, ISBN) VALUES (?, '9', ?, '', '')`,
			r.contentID, r.chapterTtl,
		); err != nil {
			t.Fatal(err)
		}
		typ := r.typ
		if typ == "" {
			typ = "highlight"
		}
		// A dogear's Text is NULL on the real device (not empty string).
		var text any = r.text
		if r.text == "" {
			text = nil
		}
		if _, err := db.Exec(
			`INSERT INTO Bookmark (BookmarkID, VolumeID, ContentID, Text, Annotation, DateCreated, Hidden, Type)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			r.bookmarkID, r.volumeID, r.contentID, text, r.annotation, r.dateCreated, r.hidden, typ,
		); err != nil {
			t.Fatal(err)
		}
	}
}
