package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ryoku/kubegate/internal/cli"
)

// baseEnvCreateArgs returns the default required flags for env create.
func baseEnvCreateArgs(apiURL string) []string {
	return []string{
		"--api-url", apiURL,
		"--product", "my-app",
		"--name", "production",
		"--type", "production",
		"--overlay", "overlays/my-app/api/production",
		"--slug", "production",
	}
}

// runEnvCreate starts a test server with handler, executes `env create` with all
// required flags plus any extraArgs, and returns captured stdout. It fails the
// test if Execute returns an error.
func runEnvCreate(t *testing.T, handler http.HandlerFunc, extraArgs ...string) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})
	cmd := cli.NewEnvCreateCmd(configDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append(baseEnvCreateArgs(srv.URL), extraArgs...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	return buf.String()
}

// runEnvCreateExpectError starts a test server with handler, executes `env create`
// with all required flags plus any extraArgs using the given token, and returns
// the execution error.
func runEnvCreateExpectError(t *testing.T, handler http.HandlerFunc, token string, extraArgs ...string) error {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cmd := cli.NewEnvCreateCmd(writeTempToken(t, cli.StoredToken{AccessToken: token}))
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append(baseEnvCreateArgs(srv.URL), extraArgs...))
	return cmd.Execute()
}

func TestEnvCreateSuccess(t *testing.T) {
	var gotPath string
	var gotBody map[string]string

	output := runEnvCreate(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc","name":"production","type":"production","overlay_path":"overlays/my-app/api/production"}`))
	})

	if gotPath != "/api/v1/products/my-app/environments" {
		t.Errorf("request path = %q, want /api/v1/products/my-app/environments", gotPath)
	}
	if gotBody["name"] != "production" {
		t.Errorf("request body name = %q, want 'production'", gotBody["name"])
	}
	if gotBody["type"] != "production" {
		t.Errorf("request body type = %q, want 'production'", gotBody["type"])
	}
	if gotBody["overlay_path"] != "overlays/my-app/api/production" {
		t.Errorf("request body overlay_path = %q, want 'overlays/my-app/api/production'", gotBody["overlay_path"])
	}
	if gotBody["slug"] != "production" {
		t.Errorf("request body slug = %q, want 'production'", gotBody["slug"])
	}
	if !strings.Contains(output, "production") {
		t.Errorf("output missing confirmation message, got: %s", output)
	}
}

func TestEnvCreateJSONOutput(t *testing.T) {
	rawJSON := `{"id":"abc","name":"production","type":"production","overlay_path":"overlays/my-app/api/production"}`
	output := runEnvCreate(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(rawJSON))
	}, "--output", "json")
	if output != rawJSON {
		t.Errorf("JSON output = %q, want %q", output, rawJSON)
	}
}

func TestEnvCreateValidationError422(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"type must be one of: dev, integration, production"}`))
	}, "test-token")
	if err == nil {
		t.Fatal("expected error on 422, got nil")
	}
	if !strings.Contains(err.Error(), "validation error") {
		t.Errorf("error = %q, want to contain 'validation error'", err.Error())
	}
	if !strings.Contains(err.Error(), "type must be one of") {
		t.Errorf("error = %q, want to contain API error message", err.Error())
	}
}

func TestEnvCreateValidationError400(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid request body"}`))
	}, "test-token")
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	if !strings.Contains(err.Error(), "validation error") {
		t.Errorf("error = %q, want to contain 'validation error'", err.Error())
	}
}

func TestEnvCreateConflict(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"environment name already exists for this product"}`))
	}, "test-token")
	if err == nil {
		t.Fatal("expected error on 409, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}

func TestEnvCreateProductNotFound(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}, "test-token")
	if err == nil {
		t.Fatal("expected error for unknown product, got nil")
	}
	if !strings.Contains(err.Error(), "product not found: my-app") {
		t.Errorf("error = %q, want to contain 'product not found: my-app'", err.Error())
	}
}

func TestEnvCreateUnauthorized(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "expired-token")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "kubegate login") {
		t.Errorf("401 error = %q, want to contain 'kubegate login'", err.Error())
	}
}

func TestEnvCreateForbidden(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}, "token")
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Errorf("403 error = %q, want to contain 'permission'", err.Error())
	}
}

func TestEnvCreateInvalidOutputFormat(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be reached for invalid output format")
	}, "token", "--output", "yaml")
	if err == nil {
		t.Fatal("expected error for unsupported output format, got nil")
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error = %q, want to contain 'yaml'", err.Error())
	}
}

func TestEnvCreateServerError(t *testing.T) {
	err := runEnvCreateExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}, "test-token")
	if err == nil {
		t.Fatal("expected error on HTTP 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message = %q, want to contain '500'", err.Error())
	}
}
