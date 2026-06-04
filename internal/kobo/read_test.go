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
	// 7 visible highlights; the one Hidden='true' row must be filtered out.
	if len(rows) != 7 {
		t.Fatalf("got %d highlights, want 7 (Hidden row excluded)", len(rows))
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
