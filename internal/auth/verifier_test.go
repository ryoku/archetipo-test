package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/ryoku/kubegate/internal/auth"
)

// localOIDCServer spins up an httptest.Server that mimics OIDC discovery and JWKS endpoints.
func localOIDCServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	pub := key.Public().(*rsa.PublicKey)
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		KeyID:     "test-key",
		Key:       pub,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}}

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"issuer":                                srv.URL,
			"jwks_uri":                              srv.URL + "/keys",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func signJWT(t *testing.T, key *rsa.PrivateKey, issuer, sub, email, name string, exp time.Time) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	claims := jwt.Claims{
		Issuer:   issuer,
		Subject:  sub,
		Expiry:   jwt.NewNumericDate(exp),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}
	extra := map[string]any{"email": email, "name": name}
	raw, err := jwt.Signed(sig).Claims(claims).Claims(extra).Serialize()
	if err != nil {
		t.Fatalf("serialize jwt: %v", err)
	}
	return raw
}

func TestVerifier_ValidToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v, err := auth.NewVerifier(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	raw := signJWT(t, key, srv.URL, "user-1", "user@example.com", "Test User", time.Now().Add(time.Hour))
	id, err := v.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id.Sub != "user-1" || id.Email != "user@example.com" || id.Name != "Test User" {
		t.Errorf("unexpected identity: %+v", id)
	}
}

func TestVerifier_ExpiredToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v, err := auth.NewVerifier(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	raw := signJWT(t, key, srv.URL, "user-1", "", "", time.Now().Add(-time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifier_TamperedSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v, err := auth.NewVerifier(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	// Signed with otherKey — the JWKS only contains key's public key.
	raw := signJWT(t, otherKey, srv.URL, "attacker", "", "", time.Now().Add(time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}

func TestVerifier_WrongIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v, err := auth.NewVerifier(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	// Token claims a different issuer than the one the verifier was initialised for.
	raw := signJWT(t, key, "https://evil.example.com", "user-1", "", "", time.Now().Add(time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestVerifier_EmptySub(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v, err := auth.NewVerifier(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	// Token with empty sub — must be rejected as the subject is a required claim.
	raw := signJWT(t, key, srv.URL, "", "user@example.com", "Alice", time.Now().Add(time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for token with empty sub")
	}
}
