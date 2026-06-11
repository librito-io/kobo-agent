package kobo

import "testing"

func TestReadHighlightSignature(t *testing.T) {
	path := writeFixtureDB(t, defaultFixtureRows())

	count, maxDate, err := ReadHighlightSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	// 8 visible rows, same set as ReadHighlights: 7 plain highlights + the
	// 'note'-typed noted highlight (#41). Hidden + dogear excluded. The two
	// queries MUST agree or the watch daemon syncs on rows the read won't send.
	if count != 8 {
		t.Fatalf("count = %d, want 8 (noted row included; Hidden + dogear excluded)", count)
	}
	// Max DateCreated among visible rows is the bk-noted row at 20:01 — 'note'
	// rows must drive the signature too (a direct note is born 'note', and the
	// watch must wake for it). The later dogear (20:05) and the Hidden row must
	// not win.
	if maxDate != "2026-06-04T20:01:02.500" {
		t.Fatalf("maxDate = %q, want 2026-06-04T20:01:02.500", maxDate)
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
