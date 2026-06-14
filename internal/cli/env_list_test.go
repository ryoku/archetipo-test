package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ryoku/kubegate/internal/cli"
)

// runEnvList starts a test server with handler, executes `env list --api-url <srv>
// --product my-app` plus any extraArgs, and returns captured stdout. It fails
// the test if Execute returns an error.
func runEnvList(t *testing.T, handler http.HandlerFunc, extraArgs ...string) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	configDir := writeTempToken(t, cli.StoredToken{AccessToken: "test-token"})
	cmd := cli.NewEnvListCmd(configDir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(append([]string{"--api-url", srv.URL, "--product", "my-app"}, extraArgs...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	return buf.String()
}

// runEnvListExpectError starts a test server with handler, executes
// `env list --api-url <srv> --product my-app` plus any extraArgs using
// the given token, suppresses stderr, and returns the execution error.
func runEnvListExpectError(t *testing.T, handler http.HandlerFunc, token string, extraArgs ...string) error {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cmd := cli.NewEnvListCmd(writeTempToken(t, cli.StoredToken{AccessToken: token}))
	cmd.SetArgs(append([]string{"--api-url", srv.URL, "--product", "my-app"}, extraArgs...))
	cmd.SetErr(&bytes.Buffer{})
	return cmd.Execute()
}

func assertEnvListHeaders(t *testing.T, output string) {
	t.Helper()
	for _, header := range []string{"NAME", "TYPE", "GITOPS PATH"} {
		if !strings.Contains(output, header) {
			t.Errorf("output missing %q header, got:\n%s", header, output)
		}
	}
}

func TestEnvListTabularOutput(t *testing.T) {
	envs := []map[string]any{
		{"name": "dev", "type": "dev", "gitops_path": "apps/dev/my-app/my-app-helmrelease.yaml"},
		{"name": "production", "type": "production", "gitops_path": "apps/production/my-app/my-app-helmrelease.yaml"},
	}
	output := runEnvList(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products/my-app/environments" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(envs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	assertEnvListHeaders(t, output)
	for _, want := range []string{
		"dev", "apps/dev/my-app/my-app-helmrelease.yaml",
		"production", "apps/production/my-app/my-app-helmrelease.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q, got:\n%s", want, output)
		}
	}
}

func TestEnvListJSONOutput(t *testing.T) {
	rawJSON := `[{"name":"dev","type":"dev","gitops_path":"apps/dev/my-app/my-app-helmrelease.yaml"}]`
	output := runEnvList(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rawJSON))
	}, "--output", "json")
	if output != rawJSON {
		t.Errorf("JSON output = %q, want %q", output, rawJSON)
	}
}

func TestEnvListProductNotFound(t *testing.T) {
	err := runEnvListExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}, "test-token")
	if err == nil {
		t.Fatal("expected error for unknown product, got nil")
	}
	if !strings.Contains(err.Error(), "product not found: my-app") {
		t.Errorf("error message = %q, want to contain 'product not found: my-app'", err.Error())
	}
}

func TestEnvListUnauthorized(t *testing.T) {
	err := runEnvListExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "expired-token")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "kubegate login") {
		t.Errorf("401 error = %q, want to contain 'kubegate login'", err.Error())
	}
}

func TestEnvListForbidden(t *testing.T) {
	err := runEnvListExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}, "token")
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Errorf("403 error = %q, want to contain 'permission'", err.Error())
	}
}

func TestEnvListServerError(t *testing.T) {
	err := runEnvListExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}, "test-token")
	if err == nil {
		t.Fatal("expected error on HTTP 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message = %q, want to contain '500'", err.Error())
	}
}

func TestEnvListInvalidOutputFormat(t *testing.T) {
	err := runEnvListExpectError(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be reached for invalid output format")
	}, "token", "--output", "csv")
	if err == nil {
		t.Fatal("expected error for unsupported output format, got nil")
	}
	if !strings.Contains(err.Error(), "csv") {
		t.Errorf("error = %q, want to contain 'csv'", err.Error())
	}
}

func TestEnvListEmptyList(t *testing.T) {
	output := runEnvList(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})
	assertEnvListHeaders(t, output)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only) for empty list, got %d lines:\n%s", len(lines), output)
	}
}
