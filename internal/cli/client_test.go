package cli_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	_ = resp.Body.Close()

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
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAPIClientGetUnreachableServer(t *testing.T) {
	// Start a server and immediately close it so the address is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// never called — server is closed immediately after creation
	}))
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
	_ = resp.Body.Close()

	if gotPath != path {
		t.Errorf("request path = %q, want %q", gotPath, path)
	}
}

func TestAPIClientPostAuthorizationHeader(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "my-post-token"}
	client := cli.NewAPIClient(srv.URL, tok)

	resp, err := client.Post(context.Background(), "/api/v1/products/x/environments", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Post returned unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	const want = "Bearer my-post-token"
	if gotAuthHeader != want {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, want)
	}
}

func TestAPIClientPostContentTypeHeader(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(srv.URL, tok)

	resp, err := client.Post(context.Background(), "/api/v1/products/x/environments", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Post returned unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	const want = "application/json"
	if gotContentType != want {
		t.Errorf("Content-Type header = %q, want %q", gotContentType, want)
	}
}

func TestAPIClientPostPathConcatenation(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(srv.URL, tok)

	const path = "/api/v1/products/my-app/environments"
	resp, err := client.Post(context.Background(), path, strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Post returned unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	if gotPath != path {
		t.Errorf("request path = %q, want %q", gotPath, path)
	}
}

func TestAPIClientPostBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(srv.URL, tok)

	const payload = `{"name":"prod","type":"production","overlay_path":"overlays/prod"}`
	resp, err := client.Post(context.Background(), "/api/v1/products/x/environments", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Post returned unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	if gotBody != payload {
		t.Errorf("request body = %q, want %q", gotBody, payload)
	}
}

func TestAPIClientPostUnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	tok := cli.StoredToken{AccessToken: "token"}
	client := cli.NewAPIClient(baseURL, tok)

	_, err := client.Post(context.Background(), "/api/v1/products/x/environments", strings.NewReader(`{}`))
	if err == nil {
		t.Fatal("expected error when server is unreachable, got nil")
	}
}
