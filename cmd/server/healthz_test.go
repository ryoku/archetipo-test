package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/router"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/domain"
)

// noopVerifier satisfies auth.TokenVerifier for tests that never reach Verify.
type noopVerifier struct{}

func (noopVerifier) Verify(_ context.Context, _ string) (*domain.UserIdentity, error) {
	return nil, nil
}

var _ auth.TokenVerifier = noopVerifier{}

func TestHealthzReturnsOKWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(noopVerifier{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /healthz without auth, got %d", w.Code)
	}
}
