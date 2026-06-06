package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ryoku/kubegate/internal/cli"
)

// runComponentList starts a test server with handler, executes `component list
// --api-url <srv> --product my-app` plus any extraArgs, and returns captured
// stdout. It fails the test if Execute returns an error.
func runComponentList(t *testing.T, handler http.HandlerFunc, extraArgs ...string) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})
	cmd := cli.NewComponentListCmd(configDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(append([]string{"--api-url", srv.URL, "--product", "my-app"}, extraArgs...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	return buf.String()
}

// runComponentListExpectError starts a test server with handler, executes
// `component list --api-url <srv> --product my-app` plus any extraArgs using
// the given token, suppresses stderr, and returns the execution error.
func runComponentListExpectError(t *testing.T, handler http.HandlerFunc, token string, extraArgs ...string) error {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cmd := cli.NewComponentListCmd(writeTempToken(t, cli.StoredToken{AccessToken: token}))
	cmd.SetArgs(append([]string{"--api-url", srv.URL, "--product", "my-app"}, extraArgs...))
	cmd.SetErr(&bytes.Buffer{})
	return cmd.Execute()
}

// assertComponentHeaders verifies that output contains all three table column headers.
func assertComponentHeaders(t *testing.T, output string) {
	t.Helper()
	for _, header := range []string{"NAME", "SLUG", "GCR IMAGE PATH"} {
		if !strings.Contains(output, header) {
			t.Errorf("output missing %q header, got:\n%s", header, output)
		}
	}
}

func TestComponentListTabularOutput(t *testing.T) {
	output := runComponentList(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products/my-app/components" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name":"Auth Service","slug":"auth-service","gcr_image_path":"gcr.io/project/auth-service"},{"name":"Gateway","slug":"gateway","gcr_image_path":"gcr.io/project/gateway"}]`))
	}))

	assertComponentHeaders(t, output)
	for _, want := range []string{
		"Auth Service", "auth-service", "gcr.io/project/auth-service",
		"Gateway", "gateway", "gcr.io/project/gateway",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q, got:\n%s", want, output)
		}
	}
}

func TestComponentListJSONOutput(t *testing.T) {
	rawJSON := `[{"name":"Auth Service","slug":"auth-service","gcr_image_path":"gcr.io/project/auth-service"}]`
	output := runComponentList(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rawJSON))
	}), "--output", "json")
	if output != rawJSON {
		t.Errorf("JSON output = %q, want %q", output, rawJSON)
	}
}

func TestComponentListProductNotFound(t *testing.T) {
	err := runComponentListExpectError(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}), "test-token")
	if err == nil {
		t.Fatal("expected error for unknown product, got nil")
	}
	if !strings.Contains(err.Error(), "product not found: my-app") {
		t.Errorf("error message = %q, want to contain 'product not found: my-app'", err.Error())
	}
}

func TestComponentListServerError(t *testing.T) {
	err := runComponentListExpectError(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}), "test-token")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message = %q, want to contain status code '500'", err.Error())
	}
}

func TestComponentListUnauthorized(t *testing.T) {
	err := runComponentListExpectError(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}), "expired-token")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "kubegate login") {
		t.Errorf("401 error = %q, want to contain 'kubegate login'", err.Error())
	}
}

func TestComponentListForbidden(t *testing.T) {
	err := runComponentListExpectError(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}), "token")
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Errorf("403 error = %q, want to contain 'permission'", err.Error())
	}
}

func TestComponentListInvalidOutputFormat(t *testing.T) {
	err := runComponentListExpectError(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}), "token", "--output", "csv")
	if err == nil {
		t.Fatal("expected error for unsupported output format, got nil")
	}
	if !strings.Contains(err.Error(), "csv") {
		t.Errorf("error = %q, want to contain 'csv'", err.Error())
	}
}

func TestComponentListEmptyList(t *testing.T) {
	output := runComponentList(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	assertComponentHeaders(t, output)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only) for empty list, got %d lines:\n%s", len(lines), output)
	}
}
