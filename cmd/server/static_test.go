package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

const (
	indexFileName     = "index.html"
	contentTypeHeader = "Content-Type"
	htmlContentType   = "text/html"
)

func newTestRouter(fixture fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerSPAFromFS(r, fixture)
	return r
}

func TestSPARootServesHTML(t *testing.T) {
	fixture := fstest.MapFS{
		indexFileName: {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get(contentTypeHeader)
	if !strings.HasPrefix(ct, htmlContentType) {
		t.Fatalf("expected text/html content-type, got %q", ct)
	}
}

func TestSPADeepPathFallsBackToIndex(t *testing.T) {
	fixture := fstest.MapFS{
		indexFileName: {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/some/deep/path", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 via SPA fallback, got %d", w.Code)
	}
	ct := w.Header().Get(contentTypeHeader)
	if !strings.HasPrefix(ct, htmlContentType) {
		t.Fatalf("expected text/html content-type for fallback, got %q", ct)
	}
}

func TestSPAStaticAssetServedDirectly(t *testing.T) {
	fixture := fstest.MapFS{
		indexFileName:    {Data: []byte(`<!doctype html>`)},
		"assets/main.js": {Data: []byte(`console.log("hello")`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/main.js", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for existing asset, got %d", w.Code)
	}
}

// TestSPADirectoryPathFallsBackToIndex guards against directory enumeration:
// Open("assets") succeeds on fstest.MapFS (implicit directory) and http.FileServer
// would issue a 301 to /assets/ revealing the directory exists, or serve a listing.
// The handler must stat the opened entry and treat directories as SPA fallbacks.
func TestSPADirectoryPathFallsBackToIndex(t *testing.T) {
	fixture := fstest.MapFS{
		indexFileName:    {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
		"assets/main.js": {Data: []byte(`console.log("hello")`)},
	}
	r := newTestRouter(fixture)

	for _, path := range []string{"/assets", "/assets/"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)

		// Must not 301 to reveal directory structure, and must not list directory contents.
		if w.Code == http.StatusMovedPermanently {
			t.Fatalf("path %q: got 301 redirect — directory existence leaked", path)
		}
		ct := w.Header().Get(contentTypeHeader)
		if !strings.HasPrefix(ct, htmlContentType) {
			t.Fatalf("path %q: expected text/html SPA fallback, got %q (status %d)", path, ct, w.Code)
		}
		if strings.Contains(w.Body.String(), "main.js") {
			t.Fatalf("path %q: directory listing leaked asset filenames", path)
		}
	}
}

// TestSPARootFallsBackToIndexNotDirectory guards against the Open("") bug:
// stripping "/" from path "/" produces "", and Open("") on a real fs.FS opens
// the root directory — not index.html. The handler must skip Open for root.
func TestSPARootFallsBackToIndexNotDirectory(t *testing.T) {
	fixture := fstest.MapFS{
		// Explicitly include the root dir entry to expose the Open("") footgun.
		".":           &fstest.MapFile{Mode: fs.ModeDir},
		indexFileName: {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get(contentTypeHeader)
	if !strings.HasPrefix(ct, htmlContentType) {
		t.Fatalf("expected text/html for root (not a directory listing), got %q", ct)
	}
}
