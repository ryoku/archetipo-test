package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/domain"
)

// mockVerifier is a test double for auth.TokenVerifier.
type mockVerifier struct {
	identity *domain.UserIdentity
	err      error
}

func (m *mockVerifier) Verify(_ context.Context, _ string) (*domain.UserIdentity, error) {
	return m.identity, m.err
}

var _ auth.TokenVerifier = (*mockVerifier)(nil)

func newTestEngine(v auth.TokenVerifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/", middleware.JWTAuth(v))
	protected.GET("/protected", func(c *gin.Context) {
		id, _ := c.Get(domain.IdentityContextKey)
		c.JSON(http.StatusOK, id)
	})
	return r
}

func TestJWTAuth_ValidToken(t *testing.T) {
	v := &mockVerifier{identity: &domain.UserIdentity{Sub: "u1", Email: "u@x.com", Name: "Alice"}}
	r := newTestEngine(v)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid.token.here")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	v := &mockVerifier{err: errors.New("token is expired")}
	r := newTestEngine(v)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired.token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Body.String() != `{"error":"unauthorized"}` {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}

func TestJWTAuth_WrongIssuer(t *testing.T) {
	v := &mockVerifier{err: errors.New("oidc: id token issued by a different provider")}
	r := newTestEngine(v)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer wrong.issuer.token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_MissingAuthorizationHeader(t *testing.T) {
	v := &mockVerifier{}
	r := newTestEngine(v)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Body.String() != `{"error":"unauthorized"}` {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}
