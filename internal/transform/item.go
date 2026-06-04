package transform

// RawBookmark is one joined row from the Kobo SQLite read:
// Bookmark JOIN content (book row, by VolumeID) LEFT JOIN content (chapter row,
// by Bookmark.ContentID). Field values are taken verbatim from the DB; all
// cleaning/validation happens in BuildItem.
type RawBookmark struct {
	BookmarkID   string // → source_uid (stable per-highlight dedup key)
	VolumeID     string // book-root content id → content_id (book_hash source)
	Text         string // raw highlighted text (carries leading whitespace)
	DateCreated  string // naive-UTC ISO, no zone designator
	Title        string // book title (content.Title on the ContentType=6 row)
	Attribution  string // book author (content.Attribution), passed raw
	ISBN         string // content.ISBN — often junk (calibre:N, ASIN, urn)
	ChapterTitle string // chapter content.Title (ContentType=9 row), may be ""
}

// KoboImportItem is the wire shape POSTed to /api/import/kobo. JSON tags mirror
// the server's KoboImportItem (src/lib/server/import/kobo.ts). Optional fields
// are pointers so a nil renders as JSON null (which the server's validator
// accepts and the RPC handles: null isbn → content_id path, null created_at →
// COALESCE to now()).
type KoboImportItem struct {
	SourceUID    string  `json:"source_uid"`
	Text         string  `json:"text"`
	ISBN         *string `json:"isbn"`
	Title        string  `json:"title"`
	Author       string  `json:"author"`
	ContentID    string  `json:"content_id"`
	ChapterTitle *string `json:"chapter_title"`
	CreatedAt    *string `json:"created_at"`
}

// BuildItem maps one joined Kobo row to the import wire shape. Returns ok=false
// when the highlight has no usable text after trimming (the row is dropped).
//
//   - content_id = VolumeID (book root), NOT the chapter-fragment Bookmark
//     ContentID — the fragment varies per chapter and would split one book into
//     many phantom book_hash rows.
//   - ISBN is shape-validated; junk (calibre:N, ASIN, urn) → nil so the book
//     resolves via content_id/book_hash instead of a bogus catalog lookup.
//   - Author is passed raw (no Last,First flipping — guessy, and ISBN is the
//     stronger catalog signal for real-ISBN books).
//   - A blank chapter title → nil. A bad/blank DateCreated → nil created_at
//     (server defaults to now()); it never drops the highlight.
func BuildItem(r RawBookmark) (KoboImportItem, bool) {
	text := CleanText(r.Text)
	if text == "" {
		return KoboImportItem{}, false
	}

	var chapter *string
	if ct := CleanText(r.ChapterTitle); ct != "" {
		chapter = &ct
	}

	return KoboImportItem{
		SourceUID:    r.BookmarkID,
		Text:         text,
		ISBN:         NormalizeISBN(r.ISBN),
		Title:        r.Title,
		Author:       r.Attribution,
		ContentID:    r.VolumeID,
		ChapterTitle: chapter,
		CreatedAt:    NormalizeTimestamp(r.DateCreated),
	}, true
}
