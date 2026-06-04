// Package transform holds the pure mapping functions that turn raw Kobo
// Bookmark/content rows into the wire shape the Librito import endpoint
// expects (POST /api/import/kobo). Kept pure + dependency-free so every
// edge case the live device data surfaced is covered by table tests.
package transform

import (
	"strings"
	"time"
)

// NormalizeTimestamp takes a Kobo Bookmark.DateCreated string and returns a
// timestamp the import endpoint can store unambiguously, or nil to let the
// server default it (the RPC COALESCEs a null created_at to now()).
//
// Kobo writes DateCreated as a naive-UTC ISO string with no zone designator
// (e.g. "2026-06-04T19:42:35.372" — verified UTC on a BST device). Fed into a
// Postgres timestamptz with no zone, that is read as server-local time and is
// wrong by the UTC offset. If the value already carries a zone ('Z' or a
// ±HH:MM offset) it is trusted as-is; otherwise 'Z' is appended.
//
// Values that don't parse as a real timestamp (empty, garbage, Kobo's
// "0000-00-00T00:00:00.000" placeholder) return nil rather than being
// forwarded — the backend would 400 the whole batch on an unparseable value.
func NormalizeTimestamp(raw string) *string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}

	if hasZoneDesignator(s) {
		if !parsesAsTimestamp(s) {
			return nil
		}
		return &s
	}

	withZ := s + "Z"
	if !parsesAsTimestamp(withZ) {
		return nil
	}
	return &withZ
}

// hasZoneDesignator reports whether s already ends in a zone marker: a literal
// Z/z, or a numeric offset like +01:00 / -05:00.
func hasZoneDesignator(s string) bool {
	if s == "" {
		return false
	}
	last := s[len(s)-1]
	if last == 'Z' || last == 'z' {
		return true
	}
	// Numeric offset: a +/- appearing after the 'T' (so the date's own
	// hyphens don't count). Offsets are the trailing 6 chars (±HH:MM).
	if t := strings.IndexByte(s, 'T'); t >= 0 {
		rest := s[t:]
		if strings.ContainsAny(rest, "+") || strings.LastIndexByte(rest, '-') > 0 {
			return true
		}
	}
	return false
}

func parsesAsTimestamp(s string) bool {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

// CleanText trims surrounding whitespace from a Kobo highlight's Text. Kobo
// stores a leading tab/space before the selected prose (seen in live data);
// internal whitespace and unicode (curly quotes) are preserved. An
// all-whitespace value collapses to "" so the caller can drop the row.
func CleanText(raw string) string {
	return strings.TrimSpace(raw)
}

// NormalizeISBN validates the shape of a Kobo content.ISBN value and returns
// the bare normalized ISBN, or nil when the value is not a real ISBN.
//
// Kobo stores whatever the epub's OPF <dc:identifier> carried, so content.ISBN
// is frequently NOT an ISBN: Calibre writes "calibre:4", sideloads carry
// "urn:uuid:…", Amazon-sourced files carry an ASIN ("B0…"). Passing those
// through as `isbn` would (a) trigger a useless catalog lookup and (b) risk two
// distinct books colliding on the same fake identifier. Rejecting them (→ nil)
// routes the book to the content_id/book_hash identity path instead.
//
// Shape rules only (no check-digit math): strip hyphens/spaces, then accept
// 13 digits (ISBN-13) or 9 digits + final digit-or-X (ISBN-10, X upper-cased).
func NormalizeISBN(raw string) *string {
	s := strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, raw)

	switch len(s) {
	case 13:
		if allDigits(s) {
			return &s
		}
	case 10:
		body, last := s[:9], s[9]
		if allDigits(body) && (isDigit(rune(last)) || last == 'X' || last == 'x') {
			out := body + strings.ToUpper(string(last))
			return &out
		}
	}
	return nil
}

func allDigits(s string) bool {
	for _, r := range s {
		if !isDigit(r) {
			return false
		}
	}
	return len(s) > 0
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }
