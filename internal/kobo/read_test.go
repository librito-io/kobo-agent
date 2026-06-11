package kobo

import "testing"

// Reads against a fabricated Kobo DB fixture (fictional books — see
// fixture_test.go). Verifies the join shape, the Hidden filter, the
// chapter-title LEFT JOIN, and that both junk and empty ISBNs come through raw.
func TestReadHighlights(t *testing.T) {
	db := writeFixtureDB(t, defaultFixtureRows())

	rows, err := ReadHighlights(db)
	if err != nil {
		t.Fatalf("ReadHighlights: %v", err)
	}
	// 8 visible rows: 7 plain highlights + the 'note'-typed noted highlight
	// (#41 — its Text is still highlighted text). Excluded: the Hidden='true'
	// row and the Text-less 'dogear'.
	if len(rows) != 8 {
		t.Fatalf("got %d rows, want 8 (noted row included; Hidden + dogear excluded)", len(rows))
	}

	byID := map[string]struct {
		title, isbn, chapter, volume string
	}{}
	for _, r := range rows {
		byID[r.BookmarkID] = struct{ title, isbn, chapter, volume string }{
			r.Title, r.ISBN, r.ChapterTitle, r.VolumeID,
		}
	}

	// The Hidden row must be absent.
	if _, ok := byID["bk-hidden"]; ok {
		t.Fatal("Hidden='true' row leaked into results")
	}

	// The noted highlight (Type='note') must be PRESENT — noting flips the row's
	// Type but its Text is still the highlighted passage (#41).
	noted, ok := byID["bk-noted"]
	if !ok {
		t.Fatal("'note'-typed highlight missing — noted highlights must sync (#41)")
	}
	if noted.chapter != "Chapter 3: Twin Logs" {
		t.Fatalf("noted row chapter = %q, want 'Chapter 3: Twin Logs'", noted.chapter)
	}

	// The dogear (page mark, NULL Text) must be absent.
	if _, ok := byID["bk-dogear"]; ok {
		t.Fatal("dogear row leaked into results")
	}

	// Real ISBN + chapter title resolved from the ContentType=9 chapter row.
	real := byID["bk-realisbn"]
	if real.title != "Glass Tide" || real.isbn != "9990000000017" {
		t.Fatalf("real-isbn row wrong: %+v", real)
	}
	if real.chapter != "Chapter 1: Salt" {
		t.Fatalf("chapter title = %q, want 'Chapter 1: Salt'", real.chapter)
	}

	// calibre:N junk ISBN comes through RAW (read layer does not validate).
	if got := byID["bk-calibre"].isbn; got != "calibre:4" {
		t.Fatalf("calibre row isbn = %q, want raw 'calibre:4'", got)
	}

	// Genuinely ISBN-less sideload: empty string through the read layer.
	if got := byID["bk-noisbn"].isbn; got != "" {
		t.Fatalf("no-isbn row isbn = %q, want empty string", got)
	}

	// VolumeID (book root, not the chapter fragment) carried on every row.
	for id, r := range byID {
		if r.volume == "" {
			t.Fatalf("row %s has empty VolumeID", id)
		}
	}
}
