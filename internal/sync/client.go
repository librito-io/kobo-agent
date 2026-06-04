// Package sync POSTs mapped Kobo highlights to the Librito import endpoint.
package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/librito-io/kobo-agent/internal/transform"
)

// ImportResult mirrors the server's KoboImportResult ({imported, books}).
type ImportResult struct {
	Imported int `json:"imported"`
	Books    int `json:"books"`
}

// serverError is the {error, message} shape jsonError returns on non-2xx.
type serverError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

const importPath = "/api/import/kobo"

// PostImport sends the full item set to {baseURL}/api/import/kobo with a device
// bearer token. The import is idempotent server-side (dedup on
// (book_id, source, source_uid)), so the agent always re-sends the whole set;
// PostImport does no diffing. An empty set is a no-op (no request) — the server
// rejects an empty array with 400, and there is nothing to sync.
//
// Go's net/http carries its own TLS stack and CA roots, so HTTPS to librito.io
// does not depend on the Kobo's (stale) system CA store or busybox ssl_client.
func PostImport(baseURL, token string, items []transform.KoboImportItem) (ImportResult, error) {
	if len(items) == 0 {
		return ImportResult{}, nil
	}

	body, err := json.Marshal(items)
	if err != nil {
		return ImportResult{}, fmt.Errorf("marshal items: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + importPath
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ImportResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ImportResult{}, fmt.Errorf("post import: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return ImportResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var se serverError
		_ = json.Unmarshal(respBody, &se)
		msg := se.Message
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		return ImportResult{}, fmt.Errorf("import failed: %d %s", resp.StatusCode, msg)
	}

	var result ImportResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ImportResult{}, fmt.Errorf("decode response: %w (%s)", err, respBody)
	}
	return result, nil
}
