package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ryoku/kubegate/internal/cli"
	"github.com/spf13/cobra"
)

const (
	notLoggedInMessage  = "Not logged in. Run `kubegate login`."
	errorMessageFormat  = "error message = %q, want %q"
	refreshedAccessToken = "new-access-token"
)

// runPreRun executes RequireAuthPreRun against a no-op command and returns
// the resulting error.
func runPreRun(t *testing.T, configDir string) error {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	hook := cli.RequireAuthPreRun(configDir)
	return hook(cmd, nil)
}

func TestRequireAuthNoToken(t *testing.T) {
	err := runPreRun(t, t.TempDir())
	if err == nil {
		t.Fatal("expected error when no token file exists")
	}
	const want = notLoggedInMessage
	if err.Error() != want {
		t.Errorf(errorMessageFormat, err.Error(), want)
	}
}

func TestRequireAuthValidToken(t *testing.T) {
	dir := t.TempDir()
	tok := cli.StoredToken{
		AccessToken: "valid",
		Expiry:      time.Now().Add(time.Hour),
	}
	if err := cli.WriteToken(dir, tok); err != nil {
		t.Fatal(err)
	}
	if err := runPreRun(t, dir); err != nil {
		t.Errorf("expected no error for valid token, got %v", err)
	}
}

func TestRequireAuthExpiredTokenRefreshSuccess(t *testing.T) {
	dir := t.TempDir()

	// Set up a mock token endpoint that returns a fresh token.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"access_token":  refreshedAccessToken,
			"token_type":    "Bearer",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	expired := cli.StoredToken{
		AccessToken:  "old",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(-time.Hour),
		IssuerURL:    srv.URL,
		ClientID:     "kubegate-cli",
		TokenURL:     srv.URL + "/token",
	}
	if err := cli.WriteToken(dir, expired); err != nil {
		t.Fatal(err)
	}

	if err := runPreRun(t, dir); err != nil {
		t.Errorf("expected successful refresh, got error: %v", err)
	}

	updated, err := cli.ReadToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AccessToken != refreshedAccessToken {
		t.Errorf("AccessToken after refresh = %q, want %q", updated.AccessToken, refreshedAccessToken)
	}
}

func TestRequireAuthExpiredTokenRefreshFailure(t *testing.T) {
	dir := t.TempDir()

	// Token endpoint that always returns an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	expired := cli.StoredToken{
		AccessToken:  "old",
		RefreshToken: "expired-refresh",
		Expiry:       time.Now().Add(-time.Hour),
		IssuerURL:    srv.URL,
		ClientID:     "kubegate-cli",
		TokenURL:     srv.URL + "/token",
	}
	if err := cli.WriteToken(dir, expired); err != nil {
		t.Fatal(err)
	}

	err := runPreRun(t, dir)
	if err == nil {
		t.Fatal("expected error when refresh fails")
	}
	const want = notLoggedInMessage
	if err.Error() != want {
		t.Errorf(errorMessageFormat, err.Error(), want)
	}
}

func TestRequireAuthExpiredNoRefreshToken(t *testing.T) {
	dir := t.TempDir()
	expired := cli.StoredToken{
		AccessToken: "old",
		Expiry:      time.Now().Add(-time.Hour),
	}
	if err := cli.WriteToken(dir, expired); err != nil {
		t.Fatal(err)
	}
	err := runPreRun(t, dir)
	if err == nil {
		t.Fatal("expected error when no refresh token is available")
	}
	const want = notLoggedInMessage
	if err.Error() != want {
		t.Errorf(errorMessageFormat, err.Error(), want)
	}
}
