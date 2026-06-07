package kobo

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// signatureQuery mirrors readQuery's INNER JOIN to the book content row and its
// visibility filters, but selects only the two scalars the watch daemon needs to
// detect new highlights: the count and the max DateCreated. Mirroring the JOIN is
// load-bearing — an orphan highlight with no book row is excluded by readQuery's
// INNER JOIN, so a naive count over Bookmark alone would drift from the synced
// set. COALESCE guards max() over zero rows (NULL → "").
const signatureQuery = `
SELECT count(*), COALESCE(max(b.DateCreated), '')
FROM Bookmark b
JOIN content c ON c.ContentID = b.VolumeID
WHERE b.Type = 'highlight'
  AND b.Hidden = 'false'
  AND b.Text IS NOT NULL
  AND trim(b.Text) <> ''`

// ReadHighlightSignature opens the KoboReader.sqlite at path read-only (WAL-aware,
// like ReadHighlights) and returns the visible-highlight count and the max
// DateCreated. It transfers two scalars — it does NOT materialize highlight rows.
func ReadHighlightSignature(path string) (count int, maxDate string, err error) {
	dsn := "file:" + url.PathEscape(path) + "?mode=ro"
	db, openErr := sql.Open("sqlite", dsn)
	if openErr != nil {
		return 0, "", fmt.Errorf("open kobo db: %w", openErr)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close kobo db: %w", cerr)
		}
	}()

	if scanErr := db.QueryRow(signatureQuery).Scan(&count, &maxDate); scanErr != nil {
		return 0, "", fmt.Errorf("scan signature: %w", scanErr)
	}
	return count, maxDate, nil
}
