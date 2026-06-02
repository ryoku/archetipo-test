package router_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/router"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/domain"
)

type alwaysDenyVerifier struct{}

func (alwaysDenyVerifier) Verify(_ context.Context, _ string) (*domain.UserIdentity, error) {
	return nil, errors.New("unauthorized")
}

var _ auth.TokenVerifier = alwaysDenyVerifier{}

func TestRouter_ProtectedEndpointReturns401WithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{}, func(api *gin.RouterGroup) {
		api.GET("/ping", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected endpoint without token, got %d", w.Code)
	}
}

func TestRouter_HealthzBypassesAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /healthz without auth, got %d", w.Code)
	}
}
