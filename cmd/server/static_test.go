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

func newTestRouter(fixture fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerSPAFromFS(r, fixture)
	return r
}

func TestSPA_RootServesHTML(t *testing.T) {
	fixture := fstest.MapFS{
		"index.html": {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html content-type, got %q", ct)
	}
}

func TestSPA_DeepPathFallsBackToIndex(t *testing.T) {
	fixture := fstest.MapFS{
		"index.html": {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/some/deep/path", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 via SPA fallback, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html content-type for fallback, got %q", ct)
	}
}

func TestSPA_StaticAssetServedDirectly(t *testing.T) {
	fixture := fstest.MapFS{
		"index.html":     {Data: []byte(`<!doctype html>`)},
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

// TestSPA_RootFallsBackToIndexNotDirectory guards against the Open("") bug:
// stripping "/" from path "/" produces "", and Open("") on a real fs.FS opens
// the root directory — not index.html. The handler must skip Open for root.
func TestSPA_RootFallsBackToIndexNotDirectory(t *testing.T) {
	fixture := fstest.MapFS{
		// Explicitly include the root dir entry to expose the Open("") footgun.
		".":          &fstest.MapFile{Mode: fs.ModeDir},
		"index.html": {Data: []byte(`<!doctype html><html><body id="root"></body></html>`)},
	}
	r := newTestRouter(fixture)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html for root (not a directory listing), got %q", ct)
	}
}
