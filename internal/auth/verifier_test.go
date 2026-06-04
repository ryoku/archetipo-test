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
	"github.com/ryoku/kubegate/internal/domain"
)

const (
	verifierTestKeyID     = "test-key"
	newVerifierErrorFmt   = "NewVerifier: %v"
	defaultUserSub        = "user-1"
	defaultUserEmail      = "user@example.com"
	audienceUserEmail     = "u@x.com"
)

type tokenIdentity struct {
	sub   string
	email string
	name  string
}

func mustNewVerifier(t *testing.T, issuerURL, clientID string) *auth.Verifier {
	t.Helper()
	v, err := auth.NewVerifier(context.Background(), issuerURL, clientID)
	if err != nil {
		t.Fatalf(newVerifierErrorFmt, err)
	}
	return v
}

// localOIDCServer spins up an httptest.Server that mimics OIDC discovery and JWKS endpoints.
func localOIDCServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	pub := key.Public().(*rsa.PublicKey)
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		KeyID:     verifierTestKeyID,
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

func signJWT(t *testing.T, key *rsa.PrivateKey, issuer string, identity tokenIdentity, exp time.Time) string {
	t.Helper()
	return signJWTWithAud(t, key, issuer, identity, exp, nil)
}

func signJWTWithAud(t *testing.T, key *rsa.PrivateKey, issuer string, identity tokenIdentity, exp time.Time, aud []string) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", verifierTestKeyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	claims := jwt.Claims{
		Issuer:   issuer,
		Subject:  identity.sub,
		Expiry:   jwt.NewNumericDate(exp),
		IssuedAt: jwt.NewNumericDate(time.Now()),
		Audience: jwt.Audience(aud),
	}
	extra := map[string]any{"email": identity.email, "name": identity.name}
	raw, err := jwt.Signed(sig).Claims(claims).Claims(extra).Serialize()
	if err != nil {
		t.Fatalf("serialize jwt: %v", err)
	}
	return raw
}

func TestVerifierValidToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v := mustNewVerifier(t, srv.URL, "")

	raw := signJWT(t, key, srv.URL, tokenIdentity{sub: defaultUserSub, email: defaultUserEmail, name: "Test User"}, time.Now().Add(time.Hour))
	id, err := v.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id.Sub != defaultUserSub || id.Email != defaultUserEmail || id.Name != "Test User" {
		t.Errorf("unexpected identity: %+v", id)
	}
}

func TestVerifierExpiredToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v := mustNewVerifier(t, srv.URL, "")

	raw := signJWT(t, key, srv.URL, tokenIdentity{sub: defaultUserSub}, time.Now().Add(-time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifierTamperedSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v := mustNewVerifier(t, srv.URL, "")

	// Signed with otherKey — the JWKS only contains key's public key.
	raw := signJWT(t, otherKey, srv.URL, tokenIdentity{sub: "attacker"}, time.Now().Add(time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}

func TestVerifierWrongIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v := mustNewVerifier(t, srv.URL, "")

	// Token claims a different issuer than the one the verifier was initialised for.
	raw := signJWT(t, key, "https://evil.example.com", tokenIdentity{sub: defaultUserSub}, time.Now().Add(time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestVerifierEmptySub(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	v := mustNewVerifier(t, srv.URL, "")

	// Token with empty sub — must be rejected as the subject is a required claim.
	raw := signJWT(t, key, srv.URL, tokenIdentity{email: defaultUserEmail, name: "Alice"}, time.Now().Add(time.Hour))
	_, err = v.Verify(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for token with empty sub")
	}
}

func TestVerifierAudienceValidatedWhenClientIDConfigured(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	const clientID = "kubegate"
	v := mustNewVerifier(t, srv.URL, clientID)

	// Token with matching aud — must be accepted.
	raw := signJWTWithAud(t, key, srv.URL, tokenIdentity{sub: defaultUserSub, email: audienceUserEmail, name: "Alice"}, time.Now().Add(time.Hour), []string{clientID})
	if _, err = v.Verify(context.Background(), raw); err != nil {
		t.Fatalf("expected success for matching aud, got: %v", err)
	}

	// Token with wrong aud — must be rejected.
	rawWrong := signJWTWithAud(t, key, srv.URL, tokenIdentity{sub: defaultUserSub, email: audienceUserEmail, name: "Alice"}, time.Now().Add(time.Hour), []string{"other-service"})
	if _, err = v.Verify(context.Background(), rawWrong); err == nil {
		t.Fatal("expected error for token with wrong aud")
	}
}

func TestVerifierAudienceSkippedWhenNoClientID(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)

	// No clientID — SkipClientIDCheck stays true, tokens without aud are accepted.
	v := mustNewVerifier(t, srv.URL, "")

	raw := signJWT(t, key, srv.URL, tokenIdentity{sub: defaultUserSub, email: audienceUserEmail, name: "Alice"}, time.Now().Add(time.Hour))
	if _, err = v.Verify(context.Background(), raw); err != nil {
		t.Fatalf("expected success without clientID, got: %v", err)
	}
}

// signJWTWithRealmRoles signs a JWT that includes a realm_access.roles claim.
func signJWTWithRealmRoles(t *testing.T, key *rsa.PrivateKey, issuer, sub string, exp time.Time, realmRoles []string) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", verifierTestKeyID),
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
	extra := map[string]any{
		"realm_access": map[string]any{"roles": realmRoles},
	}
	raw, err := jwt.Signed(sig).Claims(claims).Claims(extra).Serialize()
	if err != nil {
		t.Fatalf("serialize jwt: %v", err)
	}
	return raw
}

func TestVerifierRoleClaimsExtraction(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := localOIDCServer(t, key)
	v := mustNewVerifier(t, srv.URL, "")

	// JWT with product editor role, devops-admin, and a non-kubegate role.
	raw := signJWTWithRealmRoles(t, key, srv.URL, defaultUserSub, time.Now().Add(time.Hour),
		[]string{"kubegate:product-foo:editor", "kubegate:devops-admin", "other:ignored:role"})
	id, err := v.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !id.IsDevOpsAdmin {
		t.Error("expected IsDevOpsAdmin = true")
	}
	if id.ProductRoles["foo"] != domain.RoleEditor {
		t.Errorf("ProductRoles[foo] = %q, want %q", id.ProductRoles["foo"], domain.RoleEditor)
	}
	if _, found := id.ProductRoles["other"]; found {
		t.Error("non-kubegate role should not appear in ProductRoles")
	}

	// JWT with no kubegate-prefixed roles yields empty ProductRoles and IsDevOpsAdmin false.
	rawEmpty := signJWTWithRealmRoles(t, key, srv.URL, "user-2", time.Now().Add(time.Hour),
		[]string{"realm:user", "openid"})
	id2, err := v.Verify(context.Background(), rawEmpty)
	if err != nil {
		t.Fatalf("Verify (no kubegate roles): %v", err)
	}
	if id2.IsDevOpsAdmin {
		t.Error("expected IsDevOpsAdmin = false for JWT with no kubegate roles")
	}
	if len(id2.ProductRoles) != 0 {
		t.Errorf("expected empty ProductRoles, got %v", id2.ProductRoles)
	}
}
