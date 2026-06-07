package kobo

import "testing"

func TestReadHighlightSignature(t *testing.T) {
	path := writeFixtureDB(t, defaultFixtureRows())

	count, maxDate, err := ReadHighlightSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	// 7 visible highlights (the Hidden='true' row is excluded), same as the
	// ReadHighlights count.
	if count != 7 {
		t.Fatalf("count = %d, want 7 (Hidden row excluded)", count)
	}
	// Max DateCreated among VISIBLE rows is the bk-noisbn row at 19:55; the
	// Hidden bk-hidden row (19:50) must not win.
	if maxDate != "2026-06-04T19:55:10.120" {
		t.Fatalf("maxDate = %q, want 2026-06-04T19:55:10.120", maxDate)
	}
}

func TestReadHighlightSignature_Empty(t *testing.T) {
	path := writeFixtureDB(t, nil) // schema, no rows
	count, maxDate, err := ReadHighlightSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || maxDate != "" {
		t.Fatalf("empty db: count=%d maxDate=%q, want 0 and \"\"", count, maxDate)
	}
}
