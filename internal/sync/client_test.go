package sync

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/librito-io/kobo-agent/internal/transform"
)

func sampleItems() []transform.KoboImportItem {
	isbn := "9780593723746"
	return []transform.KoboImportItem{
		{SourceUID: "uid-1", Text: "hello", ISBN: &isbn, Title: "Book", Author: "Auth", ContentID: "file:///b.epub"},
	}
}

// Posts to a stub endpoint and verifies the request shape (method, path,
// bearer header, JSON array body) and that the parsed {imported, books}
// response is returned.
func TestPostImport_RequestShapeAndResponse(t *testing.T) {
	var gotAuth, gotPath, gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"imported":1,"books":1}`))
	}))
	defer srv.Close()

	res, err := PostImport(srv.URL, "sk_device_testtoken", sampleItems())
	if err != nil {
		t.Fatalf("PostImport: %v", err)
	}
	if res.Imported != 1 || res.Books != 1 {
		t.Fatalf("result = %+v, want imported=1 books=1", res)
	}
	if gotPath != "/api/import/kobo" {
		t.Fatalf("path = %q, want /api/import/kobo", gotPath)
	}
	if gotAuth != "Bearer sk_device_testtoken" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Fatalf("content-type = %q", gotCT)
	}
	// Body must be a JSON array of items with the wire field names.
	var arr []map[string]any
	if err := json.Unmarshal([]byte(gotBody), &arr); err != nil {
		t.Fatalf("body not a JSON array: %v\n%s", err, gotBody)
	}
	if len(arr) != 1 || arr[0]["source_uid"] != "uid-1" || arr[0]["content_id"] != "file:///b.epub" {
		t.Fatalf("body shape wrong: %s", gotBody)
	}
}

// A non-2xx response surfaces as an error carrying the status + server message,
// so the agent (and a human running it) sees why the import failed.
func TestPostImport_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized","message":"Invalid device token"}`))
	}))
	defer srv.Close()

	_, err := PostImport(srv.URL, "sk_device_bad", sampleItems())
	if err == nil {
		t.Fatal("want error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Invalid device token") {
		t.Fatalf("error should carry status + server message, got: %v", err)
	}
}

// An empty item slice is a no-op: no HTTP call, zero result. Avoids the server
// 400 on "at least one item" and saves a pointless round-trip.
func TestPostImport_EmptyIsNoOp(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	res, err := PostImport(srv.URL, "tok", nil)
	if err != nil {
		t.Fatalf("PostImport empty: %v", err)
	}
	if called {
		t.Fatal("server was called for an empty item set")
	}
	if res.Imported != 0 || res.Books != 0 {
		t.Fatalf("res = %+v, want zero", res)
	}
}
