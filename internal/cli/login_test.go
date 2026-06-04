package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ryoku/kubegate/internal/cli"
)

const (
	discoveryPath      = "/.well-known/openid-configuration"
	devicePath         = "/device"
	tokenPath          = "/token"
	contentTypeHeader  = "Content-Type"
	jsonContentType    = "application/json"
	httpScheme         = "http://"
	loginIssuerFlag    = "--issuer"
	loginClientIDFlag  = "--client-id"
	loginClientIDValue = "kubegate-cli"
	accessTokenValue   = "access-token"
	refreshTokenValue  = "refresh-token"
)

func writeJSONResponse(w http.ResponseWriter, payload any) {
	w.Header().Set(contentTypeHeader, jsonContentType)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newLoginOIDCServer(handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case discoveryPath:
			writeJSONResponse(w, oidcDiscovery(httpScheme+r.Host))
		case devicePath, tokenPath:
			handler(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

// oidcDiscovery returns a minimal OIDC discovery document pointing all
// endpoints at the given base URL.
func oidcDiscovery(base string) map[string]any {
	return map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/auth",
		"token_endpoint":                        base + tokenPath,
		"device_authorization_endpoint":         base + devicePath,
		"jwks_uri":                              base + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
}

func TestLoginSuccess(t *testing.T) {
	dir := t.TempDir()

	// First call to /device returns device auth response.
	// Second call (polling) to /token returns a token.
	var tokenCallCount int

	srv := newLoginOIDCServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case devicePath:
			writeJSONResponse(w, map[string]any{
				"device_code":      "dev-code",
				"user_code":        "USER-CODE",
				"verification_uri": "http://example.com/activate",
				"expires_in":       300,
				"interval":         1,
			})
		case tokenPath:
			tokenCallCount++
			writeJSONResponse(w, map[string]any{
				"access_token":  accessTokenValue,
				"token_type":    "Bearer",
				"refresh_token": refreshTokenValue,
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	cmd := cli.NewLoginCmd(dir)
	cmd.SetArgs([]string{loginIssuerFlag, srv.URL, loginClientIDFlag, loginClientIDValue})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login command failed: %v", err)
	}

	if tokenCallCount < 1 {
		t.Errorf("token endpoint was never called (tokenCallCount = %d)", tokenCallCount)
	}

	tok, err := cli.ReadToken(dir)
	if err != nil {
		t.Fatalf("ReadToken after login: %v", err)
	}
	if tok.AccessToken != accessTokenValue {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, accessTokenValue)
	}
	if tok.RefreshToken != refreshTokenValue {
		t.Errorf("RefreshToken = %q, want %q", tok.RefreshToken, refreshTokenValue)
	}
	if tok.IssuerURL != srv.URL {
		t.Errorf("IssuerURL = %q, want %q", tok.IssuerURL, srv.URL)
	}
	if tok.ClientID != loginClientIDValue {
		t.Errorf("ClientID = %q, want %q", tok.ClientID, loginClientIDValue)
	}
}

func TestLoginDiscoveryFailure(t *testing.T) {
	dir := t.TempDir()
	cmd := cli.NewLoginCmd(dir)
	// Point at a URL that returns an error.
	cmd.SetArgs([]string{loginIssuerFlag, "http://127.0.0.1:1", loginClientIDFlag, "test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when OIDC discovery fails, got nil")
	}
}

func TestLoginMissingIssuer(t *testing.T) {
	dir := t.TempDir()
	cmd := cli.NewLoginCmd(dir)
	cmd.SetArgs([]string{loginClientIDFlag, "test"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --issuer is missing")
	}
}

func TestLoginMissingClientID(t *testing.T) {
	dir := t.TempDir()
	cmd := cli.NewLoginCmd(dir)
	cmd.SetArgs([]string{loginIssuerFlag, "http://example.com"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --client-id is missing")
	}
}

func TestLoginNoDeviceEndpoint(t *testing.T) {
	dir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == discoveryPath {
			// Discovery doc without device_authorization_endpoint.
			writeJSONResponse(w, map[string]any{
				"issuer":                                httpScheme + r.Host,
				"authorization_endpoint":                httpScheme + r.Host + "/auth",
				"token_endpoint":                        httpScheme + r.Host + tokenPath,
				"jwks_uri":                              httpScheme + r.Host + "/jwks",
				"response_types_supported":              []string{"code"},
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cmd := cli.NewLoginCmd(dir)
	cmd.SetArgs([]string{loginIssuerFlag, srv.URL, loginClientIDFlag, loginClientIDValue})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when device_authorization_endpoint is missing")
	}
}

// Compile-time check that StoredToken.Expiry is populated from the response.
func TestLoginTokenExpirySet(t *testing.T) {
	dir := t.TempDir()

	srv := newLoginOIDCServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case devicePath:
			writeJSONResponse(w, map[string]any{
				"device_code":      "dc",
				"user_code":        "UC",
				"verification_uri": "http://example.com/activate",
				"expires_in":       300,
				"interval":         1,
			})
		case tokenPath:
			writeJSONResponse(w, map[string]any{
				"access_token":  "at",
				"token_type":    "Bearer",
				"refresh_token": "rt",
				"expires_in":    7200,
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	cmd := cli.NewLoginCmd(dir)
	cmd.SetArgs([]string{loginIssuerFlag, srv.URL, loginClientIDFlag, "c"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	tok, err := cli.ReadToken(dir)
	if err != nil {
		t.Fatalf("ReadToken after login: %v", err)
	}
	if tok.Expiry.IsZero() {
		t.Error("Expiry should not be zero after login")
	}
	if !tok.Expiry.After(time.Now()) {
		t.Errorf("Expiry %v should be in the future", tok.Expiry)
	}
}
