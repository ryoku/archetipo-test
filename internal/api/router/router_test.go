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

// assertRoutesReturn401 verifies that every (method, path) pair returns 401
// when called without a valid token, confirming all routes are registered.
func assertRoutesReturn401(t *testing.T, r *gin.Engine, endpoints [][2]string) {
	t.Helper()
	for _, e := range endpoints {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(e[0], e[1], nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401 (route exists, no auth), got %d", e[0], e[1], w.Code)
		}
	}
}

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
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodPost, "/api/v1/products"},
		{http.MethodGet, "/api/v1/products"},
		{http.MethodPut, "/api/v1/products/some-slug"},
		{http.MethodDelete, "/api/v1/products/some-slug"},
	})
}

// noopComponentStore is a no-op implementation used to verify component route registration.
type noopComponentStore struct{}

func (noopComponentStore) Create(_ context.Context, _ *domain.Component) error { return nil }
func (noopComponentStore) ListByProduct(_ context.Context, _ string) ([]domain.Component, error) {
	return nil, nil
}
func (noopComponentStore) Delete(_ context.Context, _, _ string) error { return nil }

var _ store.ComponentStore = noopComponentStore{}

// TestRegisterComponentRoutes verifies that RegisterComponentRoutes registers the
// expected HTTP endpoints. All /api/v1/* requests return 401 when no valid token is
// present, confirming the routes exist (a missing route returns 404 instead).
func TestRegisterComponentRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterComponentRoutes(noopProductStore{}, noopComponentStore{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodPost, "/api/v1/products/some-slug/components"},
		{http.MethodGet, "/api/v1/products/some-slug/components"},
		{http.MethodDelete, "/api/v1/products/some-slug/components/comp-slug"},
	})
}

// noopEnvironmentStore is a no-op implementation used to verify environment route registration.
type noopEnvironmentStore struct{}

func (noopEnvironmentStore) Create(_ context.Context, _ *domain.Environment) error { return nil }
func (noopEnvironmentStore) ListByProduct(_ context.Context, _ string) ([]domain.Environment, error) {
	return nil, nil
}
func (noopEnvironmentStore) Delete(_ context.Context, _, _ string) error { return nil }

var _ store.EnvironmentStore = noopEnvironmentStore{}

// TestRegisterEnvironmentRoutes_RoutesRegistered verifies that RegisterEnvironmentRoutes registers
// the expected HTTP endpoints. All /api/v1/* requests return 401 when no valid token is
// present, confirming the routes exist (a missing route returns 404 instead).
func TestRegisterEnvironmentRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterEnvironmentRoutes(noopProductStore{}, noopEnvironmentStore{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodPost, "/api/v1/products/some-slug/environments"},
		{http.MethodGet, "/api/v1/products/some-slug/environments"},
		{http.MethodDelete, "/api/v1/products/some-slug/environments/some-id"},
	})
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
