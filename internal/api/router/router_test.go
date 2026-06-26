package router_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/router"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/gitops"
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
func (noopProductStore) GetByID(_ context.Context, _ string) (*domain.Product, error) {
	return nil, nil
}
func (noopProductStore) Update(_ context.Context, _, _, _ string) (*domain.Product, error) {
	return nil, nil
}
func (noopProductStore) Archive(_ context.Context, _ string) error { return nil }
func (noopProductStore) GetTagConvention(_ context.Context, _ string) (*string, error) {
	return nil, nil
}
func (noopProductStore) SetTagConvention(_ context.Context, _, _ string) error { return nil }
func (noopProductStore) ClearTagConvention(_ context.Context, _ string) error  { return nil }
func (noopProductStore) ListWithStats(_ context.Context) ([]domain.ProductStats, error) {
	return nil, nil
}

var _ store.ProductStore = noopProductStore{}

type alwaysDenyVerifier struct{}

func (alwaysDenyVerifier) Verify(_ context.Context, _ string) (*domain.UserIdentity, error) {
	return nil, errors.New("unauthorized")
}

var _ auth.TokenVerifier = alwaysDenyVerifier{}

type nonAdminVerifier struct{}

func (nonAdminVerifier) Verify(_ context.Context, _ string) (*domain.UserIdentity, error) {
	return &domain.UserIdentity{Sub: "u1", Email: "u@x.com", IsDevOpsAdmin: false}, nil
}

var _ auth.TokenVerifier = nonAdminVerifier{}

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

// noopEnvironmentStore is a no-op implementation used to verify environment route registration.
type noopEnvironmentStore struct{}

func (noopEnvironmentStore) Create(_ context.Context, _ *domain.Environment) error { return nil }
func (noopEnvironmentStore) ListByProduct(_ context.Context, _ string) ([]domain.Environment, error) {
	return nil, nil
}
func (noopEnvironmentStore) GetByID(_ context.Context, _, _ string) (*domain.Environment, error) {
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

// TestRegisterTagConventionRoutes_RoutesRegistered verifies that RegisterTagConventionRoutes
// registers the expected HTTP endpoints. All /api/v1/* requests return 401 when no valid token
// is present, confirming the routes exist (a missing route returns 404 instead).
func TestRegisterTagConventionRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterTagConventionRoutes(noopProductStore{}, `^v\d+\.\d+\.\d+$`))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodGet, "/api/v1/products/some-slug/tag-convention"},
		{http.MethodPut, "/api/v1/products/some-slug/tag-convention"},
		{http.MethodDelete, "/api/v1/products/some-slug/tag-convention"},
	})
}

// noopLister is a no-op implementation used to verify tag route registration.
type noopLister struct{}

func (noopLister) ListTags(_ context.Context, _, _, _ string, _ int) ([]gcr.Tag, string, error) {
	return nil, "", nil
}

var _ gcr.Lister = noopLister{}

// noopWorkloadReader is a no-op WorkloadReader used to verify route registration.
type noopWorkloadReader struct{}

func (noopWorkloadReader) ListWorkloads(_ context.Context, _, _ string) ([]domain.Workload, error) {
	return nil, nil
}

var _ gitops.WorkloadReader = noopWorkloadReader{}

// TestRegisterTagRoutes_RoutesRegistered verifies that RegisterTagRoutes registers the
// expected HTTP endpoint. All /api/v1/* requests return 401 when no valid token is
// present, confirming the route exists (a missing route returns 404 instead).
func TestRegisterTagRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterTagRoutes(noopProductStore{}, noopEnvironmentStore{}, noopWorkloadReader{}, noopLister{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodGet, "/api/v1/products/some-slug/environments/some-id/workloads/some-workload/tags"},
	})
}

// TestRegisterWorkloadRoutes_RoutesRegistered verifies that RegisterWorkloadRoutes registers
// the expected HTTP endpoint.
func TestRegisterWorkloadRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterWorkloadRoutes(noopProductStore{}, noopEnvironmentStore{}, noopWorkloadReader{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodGet, "/api/v1/products/some-slug/environments/some-id/workloads"},
	})
}

type noopDeploymentLockStore struct{}

func (noopDeploymentLockStore) TryAcquire(_ context.Context, _, _, _ string, _ time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
	return nil, nil, nil
}
func (noopDeploymentLockStore) GetLockInfo(_ context.Context, _, _ string) (*domain.DeploymentLock, error) {
	return nil, nil
}

var _ store.DeploymentLockStore = noopDeploymentLockStore{}

type noopDeployApplier struct{}

func (noopDeployApplier) Apply(_ context.Context, _ gitops.ApplyParams) (string, error) {
	return "", nil
}

var _ handlers.GitOpsApplier = noopDeployApplier{}

type noopDeploymentStore struct{}

func (noopDeploymentStore) Create(_ context.Context, _ *domain.Deployment) error { return nil }
func (noopDeploymentStore) GetByID(_ context.Context, _ string) (*domain.Deployment, error) {
	return nil, nil
}
func (noopDeploymentStore) ListByProduct(_ context.Context, _ string, _, _ int) ([]domain.Deployment, int, error) {
	return nil, 0, nil
}
func (noopDeploymentStore) ListAll(_ context.Context, _, _ int) ([]domain.Deployment, int, error) {
	return nil, 0, nil
}
func (noopDeploymentStore) UpdateOutcome(_ context.Context, _, _ string, _ *string, _ *string) error {
	return nil
}
func (noopDeploymentStore) Delete(_ context.Context, _ string) error { return nil }
func (noopDeploymentStore) ListActivity(_ context.Context, _ int) ([]domain.Deployment, error) {
	return nil, nil
}
func (noopDeploymentStore) MarkStaleInProgress(_ context.Context, _ time.Duration) error {
	return nil
}

var _ store.DeploymentStore = noopDeploymentStore{}

// TestRegisterDeploymentRoutes_RoutesRegistered verifies that RegisterDeploymentRoutes registers
// the expected HTTP endpoints. All /api/v1/* requests return 401 when no valid token is
// present, confirming the routes exist (a missing route returns 404 instead).
func TestRegisterDeploymentRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterDeploymentRoutes(
			noopProductStore{}, noopEnvironmentStore{},
			noopDeploymentLockStore{}, noopDeploymentStore{}, noopDeployApplier{}, "",
		))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodPost, "/api/v1/products/some-slug/environments/some-id/deployments"},
		{http.MethodGet, "/api/v1/deployments/some-deployment-id"},
	})
}

type noopStatusReader struct{}

func (noopStatusReader) ReadCurrentTags(_ context.Context, _, _ string) (map[string]string, error) {
	return nil, nil
}

var _ gitops.StatusReader = noopStatusReader{}

// TestRegisterStatusRoutes_RoutesRegistered verifies that RegisterStatusRoutes registers the
// expected HTTP endpoint. All /api/v1/* requests return 401 when no valid token is present,
// confirming the route exists (a missing route returns 404 instead).
func TestRegisterStatusRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterStatusRoutes(noopProductStore{}, noopEnvironmentStore{}, noopStatusReader{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodGet, "/api/v1/products/some-slug/status"},
	})
}

// TestRegisterHistoryRoutes_RoutesRegistered verifies that RegisterHistoryRoutes registers the
// expected HTTP endpoints. All /api/v1/* requests return 401 without a valid token, confirming
// the routes exist (a missing route would return 404 instead).
func TestRegisterHistoryRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{},
		router.RegisterHistoryRoutes(noopProductStore{}, noopDeploymentStore{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodGet, "/api/v1/products/some-slug/deployments"},
		{http.MethodGet, "/api/v1/admin/deployments"},
	})
}

// TestRegisterAdminRoutes_RoutesRegistered verifies that RegisterAdminRoutes registers
// the expected HTTP endpoint. All /api/v1/* requests return 401 when no valid token is
// present, confirming the route exists (a missing route returns 404 instead).
func TestRegisterAdminRoutes_RoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(alwaysDenyVerifier{}, router.RegisterAdminRoutes(noopProductStore{}, noopDeploymentStore{}))
	assertRoutesReturn401(t, r, [][2]string{
		{http.MethodGet, "/api/v1/admin/products"},
		{http.MethodGet, "/api/v1/admin/activity"},
	})
}

func TestRegisterAdminRoutes_NonAdminReturns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := router.New(nonAdminVerifier{}, router.RegisterAdminRoutes(noopProductStore{}, noopDeploymentStore{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin user on admin endpoint, got %d", w.Code)
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
