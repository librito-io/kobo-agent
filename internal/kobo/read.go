// Package kobo reads highlights out of a Kobo's KoboReader.sqlite.
package kobo

import (
	"database/sql"
	"fmt"
	"net/url"

	"github.com/librito-io/kobo-agent/internal/transform"

	_ "modernc.org/sqlite"
)

// RawBookmark is the joined read-row shape; aliased from the transform package
// so the read layer and the mapping layer agree on one type.
type RawBookmark = transform.RawBookmark

// readQuery joins each highlight Bookmark to its book content row (by VolumeID,
// the ContentType=6 book root) for title/author/ISBN, and LEFT JOINs the
// chapter content row (by Bookmark.ContentID, a ContentType=9 row) for the
// chapter title. Hidden rows (device-side deleted) and empty-text rows are
// excluded. COALESCE guards NULL text/metadata so scans never hit a NULL.
const readQuery = `
SELECT
  b.BookmarkID,
  b.VolumeID,
  COALESCE(b.Text, ''),
  COALESCE(b.DateCreated, ''),
  COALESCE(c.Title, ''),
  COALESCE(c.Attribution, ''),
  COALESCE(c.ISBN, ''),
  COALESCE(ch.Title, '')
FROM Bookmark b
JOIN content c        ON c.ContentID = b.VolumeID
LEFT JOIN content ch  ON ch.ContentID = b.ContentID
WHERE b.Type = 'highlight'
  AND b.Hidden = 'false'
  AND b.Text IS NOT NULL
  AND trim(b.Text) <> ''
ORDER BY c.Title, b.DateCreated`

// ReadHighlights opens the KoboReader.sqlite at path and returns every visible
// highlight joined to its book + chapter metadata.
//
// The DB is opened read-only. SQLite is WAL-mode on a Kobo, so opening the live
// file (with its -wal / -shm siblings present) transparently includes rows that
// Nickel has written to the WAL but not yet checkpointed into the main file —
// the agent must never read a stale pre-checkpoint snapshot. `mode=ro` keeps
// the agent from mutating the user's DB or forcing a checkpoint.
func ReadHighlights(path string) ([]RawBookmark, error) {
	// Escape the path so URI-special chars in it (# truncating as a fragment,
	// ?/& injecting DSN params that could override mode=ro) are encoded as path
	// data rather than interpreted. mode=ro is appended as a real query param.
	dsn := "file:" + url.PathEscape(path) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open kobo db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(readQuery)
	if err != nil {
		return nil, fmt.Errorf("query highlights: %w", err)
	}
	defer rows.Close()

	var out []RawBookmark
	for rows.Next() {
		var r RawBookmark
		if err := rows.Scan(
			&r.BookmarkID, &r.VolumeID, &r.Text, &r.DateCreated,
			&r.Title, &r.Attribution, &r.ISBN, &r.ChapterTitle,
		); err != nil {
			return nil, fmt.Errorf("scan highlight: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate highlights: %w", err)
	}
	return out, nil
}
