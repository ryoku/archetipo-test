package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryoku/kubegate/internal/cli"
)

func TestAPIClientGetAuthorizationHeader(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "my-access-token"}
	client := cli.NewAPIClient(srv.URL, tok)

	resp, err := client.Get(context.Background(), "/api/v1/products")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	resp.Body.Close()

	const want = "Bearer my-access-token"
	if gotAuthHeader != want {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, want)
	}
}

func TestAPIClientGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(srv.URL, tok)

	resp, err := client.Get(context.Background(), "/api/v1/products")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAPIClientGetUnreachableServer(t *testing.T) {
	// Start a server and immediately close it so the address is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(baseURL, tok)

	_, err := client.Get(context.Background(), "/api/v1/products")
	if err == nil {
		t.Fatal("expected error when server is unreachable, got nil")
	}
}

func TestAPIClientGetPathConcatenation(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(srv.URL, tok)

	const path = "/api/v1/components"
	resp, err := client.Get(context.Background(), path)
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	resp.Body.Close()

	if gotPath != path {
		t.Errorf("request path = %q, want %q", gotPath, path)
	}
}
