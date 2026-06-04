package transform

import "testing"

func TestNormalizeTimestamp(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // "" means nil (omit → RPC COALESCEs to now())
	}{
		// Kobo writes DateCreated as naive-UTC with NO zone designator
		// (verified on a BST device: 20:42 local stored as 19:42). Without a
		// 'Z' it would be read as server-local by Postgres timestamptz → wrong
		// by the UTC offset. Append 'Z'.
		{"naive utc millis", "2026-06-04T19:42:35.372", "2026-06-04T19:42:35.372Z"},
		{"naive utc whole sec", "2026-06-02T19:36:33.000", "2026-06-02T19:36:33.000Z"},
		// Already carries a zone designator → leave untouched.
		{"already Z", "2026-06-02T19:36:33Z", "2026-06-02T19:36:33Z"},
		{"positive offset", "2026-06-04T19:42:35+01:00", "2026-06-04T19:42:35+01:00"},
		{"negative offset", "2026-06-04T19:42:35-05:00", "2026-06-04T19:42:35-05:00"},
		// Lowercase 'z' is NOT a valid RFC3339 zone designator (and Kobo never
		// emits it — real rows use uppercase Z). Treated as unparseable → nil.
		{"lowercase z is invalid", "2026-06-02T19:36:33z", ""},
		// Surrounding whitespace trimmed before the zone check.
		{"trims whitespace", "  2026-06-04T19:42:35.372  ", "2026-06-04T19:42:35.372Z"},
		// Empty / unusable → nil so the RPC defaults to now() instead of
		// forwarding an unparseable value (backend would 400 the batch).
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"not a timestamp", "garbage", ""},
		{"kobo epoch zero placeholder", "0000-00-00T00:00:00.000", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeTimestamp(c.in)
			if c.want == "" {
				if got != nil {
					t.Fatalf("NormalizeTimestamp(%q) = %q, want nil", c.in, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("NormalizeTimestamp(%q) = nil, want %q", c.in, c.want)
			}
			if *got != c.want {
				t.Fatalf("NormalizeTimestamp(%q) = %q, want %q", c.in, *got, c.want)
			}
		})
	}
}

func strp(s string) *string { return &s }

func TestBuildItem(t *testing.T) {
	t.Run("real isbn book maps fully", func(t *testing.T) {
		raw := RawBookmark{
			BookmarkID:   "4595c543-1d9a-48d1-89a0-592bf98ee045",
			VolumeID:     "file:///mnt/onboard/Books/Half His Age - Jennette McCurdy.epub",
			Text:         "\t I’m not used to this new body yet",
			DateCreated:  "2026-06-02T19:36:33.000",
			Title:        "Half His Age",
			Attribution:  "Jennette McCurdy",
			ISBN:         "9780593723746",
			ChapterTitle: "Chapter 1",
		}
		got, ok := BuildItem(raw)
		if !ok {
			t.Fatal("BuildItem returned ok=false, want true")
		}
		want := KoboImportItem{
			SourceUID:    "4595c543-1d9a-48d1-89a0-592bf98ee045",
			Text:         "I’m not used to this new body yet",
			ISBN:         strp("9780593723746"),
			Title:        "Half His Age",
			Author:       "Jennette McCurdy",
			ContentID:    "file:///mnt/onboard/Books/Half His Age - Jennette McCurdy.epub",
			ChapterTitle: strp("Chapter 1"),
			CreatedAt:    strp("2026-06-02T19:36:33.000Z"),
		}
		assertItem(t, got, want)
	})

	t.Run("calibre junk isbn becomes nil isbn", func(t *testing.T) {
		raw := RawBookmark{
			BookmarkID:  "725b325e-3f52-48ad-a001-fc2ca9c95ee2",
			VolumeID:    "file:///mnt/onboard/Books/Against Everything.epub",
			Text:        "Some highlighted line",
			DateCreated: "2026-06-04T19:41:53.758",
			Title:       "Against Everything",
			Attribution: "Mark Greif",
			ISBN:        "calibre:4",
		}
		got, ok := BuildItem(raw)
		if !ok {
			t.Fatal("ok=false, want true")
		}
		if got.ISBN != nil {
			t.Fatalf("ISBN = %q, want nil (junk → content_id path)", *got.ISBN)
		}
		// content_id must still carry the book-root VolumeID so the backend can
		// synthesize a stable book_hash for the sideload.
		if got.ContentID != "file:///mnt/onboard/Books/Against Everything.epub" {
			t.Fatalf("ContentID = %q", got.ContentID)
		}
	})

	t.Run("author passed raw (Last,First not flipped)", func(t *testing.T) {
		raw := RawBookmark{
			BookmarkID: "bd7a28fe", VolumeID: "file:///x.epub", Text: "t",
			DateCreated: "2026-06-04T19:32:39.811",
			Title:       "The Dream Hotel", Attribution: "Lalami,Laila",
			ISBN: "9780593317600",
		}
		got, _ := BuildItem(raw)
		if got.Author != "Lalami,Laila" {
			t.Fatalf("Author = %q, want raw 'Lalami,Laila'", got.Author)
		}
	})

	t.Run("empty text after trim drops the row", func(t *testing.T) {
		raw := RawBookmark{
			BookmarkID: "x", VolumeID: "file:///x.epub", Text: "  \t ",
			DateCreated: "2026-06-04T19:32:39.811", Title: "T", Attribution: "A",
			ISBN: "9780593317600",
		}
		if _, ok := BuildItem(raw); ok {
			t.Fatal("ok=true, want false (empty text must drop)")
		}
	})

	t.Run("missing chapter title → nil", func(t *testing.T) {
		raw := RawBookmark{
			BookmarkID: "x", VolumeID: "file:///x.epub", Text: "t",
			DateCreated: "2026-06-04T19:32:39.811", Title: "T", Attribution: "A",
			ISBN: "9780593317600", ChapterTitle: "",
		}
		got, _ := BuildItem(raw)
		if got.ChapterTitle != nil {
			t.Fatalf("ChapterTitle = %q, want nil", *got.ChapterTitle)
		}
	})

	t.Run("unparseable date → nil created_at (server defaults now)", func(t *testing.T) {
		raw := RawBookmark{
			BookmarkID: "x", VolumeID: "file:///x.epub", Text: "t",
			DateCreated: "0000-00-00T00:00:00.000", Title: "T", Attribution: "A",
			ISBN: "9780593317600",
		}
		got, ok := BuildItem(raw)
		if !ok {
			t.Fatal("ok=false; a bad date must not drop the highlight")
		}
		if got.CreatedAt != nil {
			t.Fatalf("CreatedAt = %q, want nil", *got.CreatedAt)
		}
	})
}

func assertItem(t *testing.T, got, want KoboImportItem) {
	t.Helper()
	if got.SourceUID != want.SourceUID || got.Text != want.Text ||
		got.Title != want.Title || got.Author != want.Author ||
		got.ContentID != want.ContentID {
		t.Fatalf("scalar mismatch:\n got %+v\nwant %+v", got, want)
	}
	assertStrp(t, "ISBN", got.ISBN, want.ISBN)
	assertStrp(t, "ChapterTitle", got.ChapterTitle, want.ChapterTitle)
	assertStrp(t, "CreatedAt", got.CreatedAt, want.CreatedAt)
}

func assertStrp(t *testing.T, field string, got, want *string) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Fatalf("%s: got %v want %v", field, got, want)
	case *got != *want:
		t.Fatalf("%s: got %q want %q", field, *got, *want)
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Real Kobo highlight: leading tab + space before the prose.
		{"leading tab and space", "\t I’m not used to this new body yet", "I’m not used to this new body yet"},
		// Internal unicode (curly apostrophe) + internal whitespace preserved.
		{"preserves internal", "It’s like  a  dream", "It’s like  a  dream"},
		{"trailing newline", "Imagine going to sleep\n", "Imagine going to sleep"},
		{"both ends", "  hello world  ", "hello world"},
		{"already clean", "clean", "clean"},
		{"whitespace only", "  \t\n ", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CleanText(c.in); got != c.want {
				t.Fatalf("CleanText(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeISBN(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // "" means nil (no valid ISBN)
	}{
		// Real ISBN-13s from the live Kobo DB.
		{"isbn13 plain", "9780593723746", "9780593723746"},
		{"isbn13 another", "9781668082485", "9781668082485"},
		// Calibre junk identifier slurped into content.ISBN — must be rejected
		// so the book falls to the content_id/book_hash path, not a bogus
		// catalog lookup keyed on "calibre:4".
		{"calibre junk", "calibre:4", ""},
		// ISBN-10 with X check digit (valid terminal char).
		{"isbn10 with X", "080442957X", "080442957X"},
		{"isbn10 lowercase x normalized", "080442957x", "080442957X"},
		// Hyphenated / spaced ISBN-13 normalizes to bare digits.
		{"isbn13 hyphenated", "978-0-593-72374-6", "9780593723746"},
		{"isbn13 spaced", "978 0 593 72374 6", "9780593723746"},
		// Garbage / wrong length → rejected.
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"too short", "12345", ""},
		{"urn uuid", "urn:uuid:abc", ""},
		{"asin", "B0ABCD1234", ""},
		// 13 chars but not all digits → rejected (X only legal in ISBN-10 terminal).
		{"isbn13 with letter", "97805937X3746", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeISBN(c.in)
			if c.want == "" {
				if got != nil {
					t.Fatalf("NormalizeISBN(%q) = %q, want nil", c.in, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("NormalizeISBN(%q) = nil, want %q", c.in, c.want)
			}
			if *got != c.want {
				t.Fatalf("NormalizeISBN(%q) = %q, want %q", c.in, *got, c.want)
			}
		})
	}
}
