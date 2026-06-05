package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ryoku/kubegate/internal/cli"
)

func TestComponentListTabularOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products/my-app/components" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"name":"Auth Service","slug":"auth-service","gcr_image_path":"gcr.io/project/auth-service"},{"name":"Gateway","slug":"gateway","gcr_image_path":"gcr.io/project/gateway"}]`))
	}))
	defer srv.Close()

	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})

	cmd := cli.NewComponentListCmd(configDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--api-url", srv.URL, "--product", "my-app"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "NAME") {
		t.Errorf("output missing NAME header, got:\n%s", output)
	}
	if !strings.Contains(output, "SLUG") {
		t.Errorf("output missing SLUG header, got:\n%s", output)
	}
	if !strings.Contains(output, "GCR IMAGE PATH") {
		t.Errorf("output missing GCR IMAGE PATH header, got:\n%s", output)
	}
	if !strings.Contains(output, "Auth Service") {
		t.Errorf("output missing component name 'Auth Service', got:\n%s", output)
	}
	if !strings.Contains(output, "auth-service") {
		t.Errorf("output missing component slug 'auth-service', got:\n%s", output)
	}
	if !strings.Contains(output, "gcr.io/project/auth-service") {
		t.Errorf("output missing gcr image path 'gcr.io/project/auth-service', got:\n%s", output)
	}
	if !strings.Contains(output, "Gateway") {
		t.Errorf("output missing component name 'Gateway', got:\n%s", output)
	}
	if !strings.Contains(output, "gateway") {
		t.Errorf("output missing component slug 'gateway', got:\n%s", output)
	}
	if !strings.Contains(output, "gcr.io/project/gateway") {
		t.Errorf("output missing gcr image path 'gcr.io/project/gateway', got:\n%s", output)
	}
}

func TestComponentListJSONOutput(t *testing.T) {
	rawJSON := `[{"name":"Auth Service","slug":"auth-service","gcr_image_path":"gcr.io/project/auth-service"}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rawJSON))
	}))
	defer srv.Close()

	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})

	cmd := cli.NewComponentListCmd(configDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--api-url", srv.URL, "--product", "my-app", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	output := buf.String()
	if output != rawJSON {
		t.Errorf("JSON output = %q, want %q", output, rawJSON)
	}
}

func TestComponentListProductNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})

	cmd := cli.NewComponentListCmd(configDir)
	cmd.SetArgs([]string{"--api-url", srv.URL, "--product", "my-app"})
	// Suppress error output so test output stays clean.
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown product, got nil")
	}
	if !strings.Contains(err.Error(), "product not found: my-app") {
		t.Errorf("error message = %q, want to contain 'product not found: my-app'", err.Error())
	}
}

func TestComponentListServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})

	cmd := cli.NewComponentListCmd(configDir)
	cmd.SetArgs([]string{"--api-url", srv.URL, "--product", "my-app"})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message = %q, want to contain status code '500'", err.Error())
	}
}

func TestComponentListEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})

	cmd := cli.NewComponentListCmd(configDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--api-url", srv.URL, "--product", "my-app"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("output missing NAME header for empty list, got:\n%s", output)
	}
	if !strings.Contains(output, "SLUG") {
		t.Errorf("output missing SLUG header for empty list, got:\n%s", output)
	}
	if !strings.Contains(output, "GCR IMAGE PATH") {
		t.Errorf("output missing GCR IMAGE PATH header for empty list, got:\n%s", output)
	}
	// There should be exactly one line (the header) beyond the final newline.
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only) for empty list, got %d lines:\n%s", len(lines), output)
	}
}
