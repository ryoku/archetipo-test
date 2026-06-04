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
	"github.com/ryoku/kubegate/internal/store"
)

// noopProductStore is a no-op implementation used to verify route registration
// without requiring a real database.
type noopProductStore struct{}

func (noopProductStore) Create(_ context.Context, _ *domain.Product) error { return nil }
func (noopProductStore) List(_ context.Context, _ store.ListOptions) ([]domain.Product, error) {
	return nil, nil
}
func (noopProductStore) GetBySlug(_ context.Context, _ string) (*domain.Product, error) {
	return nil, nil
}
func (noopProductStore) Update(_ context.Context, _, _, _ string) (*domain.Product, error) {
	return nil, nil
}
func (noopProductStore) Archive(_ context.Context, _ string) error { return nil }

var _ store.ProductStore = noopProductStore{}

type alwaysDenyVerifier struct{}

func (alwaysDenyVerifier) Verify(_ context.Context, _ string) (*domain.UserIdentity, error) {
	return nil, errors.New("unauthorized")
}

var _ auth.TokenVerifier = alwaysDenyVerifier{}

func TestRouterProtectedEndpointReturns401WithoutToken(t *testing.T) {
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

// TestRegisterProductRoutes verifies that RegisterProductRoutes registers the
// expected HTTP endpoints. Because alwaysDenyVerifier rejects every token, all
// /api/v1/* requests return 401, confirming the routes exist (a missing route
// would return 404 instead).
func TestRegisterProductRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{}, router.RegisterProductRoutes(noopProductStore{}))

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/products"},
		{http.MethodGet, "/api/v1/products"},
		{http.MethodPut, "/api/v1/products/some-slug"},
		{http.MethodDelete, "/api/v1/products/some-slug"},
	}
	for _, e := range endpoints {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(e.method, e.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401 (route exists, no auth), got %d", e.method, e.path, w.Code)
		}
	}
}

func TestRouterHealthzBypassesAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /healthz without auth, got %d", w.Code)
	}
}
