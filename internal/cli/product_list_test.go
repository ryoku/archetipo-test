package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryoku/kubegate/internal/cli"
	"github.com/spf13/cobra"
)

// writeTempToken creates a temporary config directory with a token.json file
// containing the given StoredToken and returns the directory path.
// This helper is shared by all cli_test files in this package.
func writeTempToken(t *testing.T, tok cli.StoredToken) string {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("writeTempToken: marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "token.json"), data, 0600); err != nil {
		t.Fatalf("writeTempToken: write: %v", err)
	}
	return dir
}

// executeProductList executes the given cobra command and returns the combined
// stdout/stderr output and any RunE error.
func executeProductList(t *testing.T, cmd *cobra.Command, args []string) (stdout string, err error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return buf.String(), err
}

func TestProductListTabularOutput(t *testing.T) {
	products := []map[string]any{
		{"name": "Alpha", "slug": "alpha", "description": "First product"},
		{"name": "Beta", "slug": "beta", "description": "Second product"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(products); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "test-token"}
	dir := writeTempToken(t, tok)

	cmd := cli.NewProductListCmd(dir)
	out, err := executeProductList(t, cmd, []string{"--api-url", srv.URL})
	if err != nil {
		t.Fatalf("command returned unexpected error: %v", err)
	}

	// Verify header row is present.
	if !strings.Contains(out, "NAME") {
		t.Errorf("output missing NAME column header; got:\n%s", out)
	}
	if !strings.Contains(out, "SLUG") {
		t.Errorf("output missing SLUG column header; got:\n%s", out)
	}
	if !strings.Contains(out, "DESCRIPTION") {
		t.Errorf("output missing DESCRIPTION column header; got:\n%s", out)
	}

	// Verify product data rows.
	if !strings.Contains(out, "Alpha") {
		t.Errorf("output missing product name 'Alpha'; got:\n%s", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("output missing product slug 'alpha'; got:\n%s", out)
	}
	if !strings.Contains(out, "First product") {
		t.Errorf("output missing description 'First product'; got:\n%s", out)
	}
	if !strings.Contains(out, "Beta") {
		t.Errorf("output missing product name 'Beta'; got:\n%s", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("output missing product slug 'beta'; got:\n%s", out)
	}
	if !strings.Contains(out, "Second product") {
		t.Errorf("output missing description 'Second product'; got:\n%s", out)
	}
}

func TestProductListJSONOutput(t *testing.T) {
	rawBody := `[{"name":"Alpha","slug":"alpha","description":"First product"}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rawBody))
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "test-token"}
	dir := writeTempToken(t, tok)

	cmd := cli.NewProductListCmd(dir)
	out, err := executeProductList(t, cmd, []string{"--api-url", srv.URL, "--output", "json"})
	if err != nil {
		t.Fatalf("command returned unexpected error: %v", err)
	}

	// Output should be the raw JSON body, not a formatted table.
	if !strings.Contains(out, rawBody) {
		t.Errorf("JSON output does not contain raw body\ngot:  %s\nwant: %s", out, rawBody)
	}

	// Verify it is valid JSON.
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v\ngot: %s", err, out)
	}
}

func TestProductListServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "test-token"}
	dir := writeTempToken(t, tok)

	cmd := cli.NewProductListCmd(dir)
	_, err := executeProductList(t, cmd, []string{"--api-url", srv.URL})
	if err == nil {
		t.Fatal("expected error on HTTP 500 response, got nil")
	}
}

func TestProductListEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "test-token"}
	dir := writeTempToken(t, tok)

	cmd := cli.NewProductListCmd(dir)
	out, err := executeProductList(t, cmd, []string{"--api-url", srv.URL})
	if err != nil {
		t.Fatalf("command returned unexpected error: %v", err)
	}

	// Only the header row should be present, no data rows.
	if !strings.Contains(out, "NAME") {
		t.Errorf("output missing NAME column header on empty list; got:\n%s", out)
	}
	if !strings.Contains(out, "SLUG") {
		t.Errorf("output missing SLUG column header on empty list; got:\n%s", out)
	}
	if !strings.Contains(out, "DESCRIPTION") {
		t.Errorf("output missing DESCRIPTION column header on empty list; got:\n%s", out)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 line (header only) for empty list, got %d lines:\n%s", len(lines), out)
	}
}
