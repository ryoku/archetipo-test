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

// executeProductList executes the given cobra command and returns the stdout
// output and any RunE error. stderr is discarded to keep test output clean.
func executeProductList(t *testing.T, cmd *cobra.Command, args []string) (stdout string, err error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

// runProductList starts a test server with handler, executes `product list --api-url <srv>`
// plus any extraArgs, and returns the captured stdout. It fails the test if Execute returns
// an error.
func runProductList(t *testing.T, handler http.HandlerFunc, extraArgs ...string) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	dir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})
	cmd := cli.NewProductListCmd(dir)
	out, err := executeProductList(t, cmd, append([]string{"--api-url", srv.URL}, extraArgs...))
	if err != nil {
		t.Fatalf("command returned unexpected error: %v", err)
	}
	return out
}

// assertProductTableHeaders verifies that output contains all three table column headers.
func assertProductTableHeaders(t *testing.T, out string) {
	t.Helper()
	for _, header := range []string{"NAME", "SLUG", "DESCRIPTION"} {
		if !strings.Contains(out, header) {
			t.Errorf("output missing %q column header; got:\n%s", header, out)
		}
	}
}

func TestProductListTabularOutput(t *testing.T) {
	products := []map[string]any{
		{"name": "Alpha", "slug": "alpha", "description": "First product"},
		{"name": "Beta", "slug": "beta", "description": "Second product"},
	}
	out := runProductList(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(products); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))

	assertProductTableHeaders(t, out)
	for _, want := range []string{"Alpha", "alpha", "First product", "Beta", "beta", "Second product"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
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

func TestProductListUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cmd := cli.NewProductListCmd(writeTempToken(t, cli.StoredToken{AccessToken: "expired-token"}))
	_, err := executeProductList(t, cmd, []string{"--api-url", srv.URL})
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "kubegate login") {
		t.Errorf("401 error message = %q, want to contain 'kubegate login'", err.Error())
	}
}

func TestProductListForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	cmd := cli.NewProductListCmd(writeTempToken(t, cli.StoredToken{AccessToken: "token"}))
	_, err := executeProductList(t, cmd, []string{"--api-url", srv.URL})
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Errorf("403 error message = %q, want to contain 'permission'", err.Error())
	}
}

func TestProductListInvalidOutputFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	cmd := cli.NewProductListCmd(writeTempToken(t, cli.StoredToken{AccessToken: "token"}))
	_, err := executeProductList(t, cmd, []string{"--api-url", srv.URL, "--output", "yaml"})
	if err == nil {
		t.Fatal("expected error for unsupported output format, got nil")
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error message = %q, want to contain 'yaml'", err.Error())
	}
}

func TestProductListEmptyList(t *testing.T) {
	out := runProductList(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))

	assertProductTableHeaders(t, out)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 line (header only) for empty list, got %d lines:\n%s", len(lines), out)
	}
}
