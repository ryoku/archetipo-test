package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock AcquiredLock ---

type mockAcquiredLock struct{}

func (m *mockAcquiredLock) Release(_ context.Context) error { return nil }

var _ store.AcquiredLock = (*mockAcquiredLock)(nil)

// --- mock DeploymentLockStore ---

type mockDeploymentLockStore struct {
	tryAcquireFn  func(ctx context.Context, productID, envID, actor string, timeout time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error)
	getLockInfoFn func(ctx context.Context, productID, envID string) (*domain.DeploymentLock, error)
}

func (m *mockDeploymentLockStore) TryAcquire(ctx context.Context, productID, envID, actor string, timeout time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
	return m.tryAcquireFn(ctx, productID, envID, actor, timeout)
}
func (m *mockDeploymentLockStore) GetLockInfo(ctx context.Context, productID, envID string) (*domain.DeploymentLock, error) {
	if m.getLockInfoFn != nil {
		return m.getLockInfoFn(ctx, productID, envID)
	}
	return nil, nil
}

var _ store.DeploymentLockStore = (*mockDeploymentLockStore)(nil)

// --- mock GitOpsApplier ---

type mockGitOpsApplier struct {
	applyFn func(ctx context.Context, p gitops.ApplyParams) error
}

func (m *mockGitOpsApplier) Apply(ctx context.Context, p gitops.ApplyParams) error {
	return m.applyFn(ctx, p)
}

var _ handlers.GitOpsApplier = (*mockGitOpsApplier)(nil)

// --- test router helper ---

func newDeployRouter(
	ps store.ProductStore,
	es store.EnvironmentStore,
	cs store.ComponentStore,
	ls store.DeploymentLockStore,
	applier handlers.GitOpsApplier,
	identity *domain.UserIdentity,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(ps, es, cs, ls, applier)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	api.POST("/products/:productSlug/environments/:environmentID/deployments",
		middleware.RequireRole(domain.RoleEditor), h.Deploy)
	return r
}

// --- fixtures ---

var deployFixtureProduct = &domain.Product{
	ID:   "prod-id-1",
	Name: "My Service",
	Slug: "my-service",
}

var deployFixtureEnv = &domain.Environment{
	ID:          "env-id-1",
	ProductID:   "prod-id-1",
	Name:        "dev",
	Type:        "dev",
	OverlayPath: "apps/dev/my-service/my-service-helmrelease.yaml",
}

var deployFixtureComp = &domain.Component{
	ID:           "comp-id-1",
	ProductID:    "prod-id-1",
	Name:         "my-service",
	Slug:         "my-service",
	GCRImagePath: "europe-docker.pkg.dev/proj/repo/my-service",
}

func editorIdentityForDeploy(productSlug string) *domain.UserIdentity {
	return &domain.UserIdentity{
		Sub:          "user-1",
		Email:        "sara@example.com",
		Name:         "Sara DevOps",
		ProductRoles: map[string]domain.Role{productSlug: domain.RoleEditor},
	}
}

func productStoreWithProduct(p *domain.Product) *mockProductStore {
	return &mockProductStore{
		getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
			if slug == p.Slug {
				return p, nil
			}
			return nil, store.ErrNotFound
		},
	}
}

func envStoreWithEnv(e *domain.Environment) *mockEnvironmentStore {
	return &mockEnvironmentStore{
		getByIDFn: func(_ context.Context, productID, environmentID string) (*domain.Environment, error) {
			if productID == e.ProductID && environmentID == e.ID {
				return e, nil
			}
			return nil, store.ErrEnvironmentNotFound
		},
	}
}

func compStoreWithComp(comp *domain.Component) *mockComponentStore {
	return &mockComponentStore{
		getBySlugFn: func(_ context.Context, productID, slug string) (*domain.Component, error) {
			if productID == comp.ProductID && slug == comp.Slug {
				return comp, nil
			}
			return nil, store.ErrComponentNotFound
		},
	}
}

func acquiringLockStore() *mockDeploymentLockStore {
	return &mockDeploymentLockStore{
		tryAcquireFn: func(_ context.Context, _, _, _ string, _ time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
			return &mockAcquiredLock{}, nil, nil
		},
	}
}

func heldLockStore(holder *domain.DeploymentLock) *mockDeploymentLockStore {
	return &mockDeploymentLockStore{
		tryAcquireFn: func(_ context.Context, _, _, _ string, _ time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
			return nil, holder, nil
		},
	}
}

func successApplier() *mockGitOpsApplier {
	return &mockGitOpsApplier{
		applyFn: func(_ context.Context, _ gitops.ApplyParams) error { return nil },
	}
}

func doDeployRequest(t *testing.T, r *gin.Engine, productSlug, envID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/products/"+productSlug+"/environments/"+envID+"/deployments",
		bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestDeploy_Success(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(),
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"component_slug": "my-service",
		"tag":            "v1.2.3",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "v1.2.3", resp["tag"])
	assert.Equal(t, "Sara DevOps", resp["deployed_by"])
}

func TestDeploy_LockHeld_Returns409WithHolderInfo(t *testing.T) {
	lockedSince := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	holder := &domain.DeploymentLock{
		ProductID:   "prod-id-1",
		EnvID:       "env-id-1",
		LockHolder:  "Marco Tech Lead",
		LockedSince: lockedSince,
	}

	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		heldLockStore(holder),
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"component_slug": "my-service",
		"tag":            "v1.2.3",
	})

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "deployment in progress", resp["error"])
	assert.Equal(t, "Marco Tech Lead", resp["lock_holder"])
	assert.Equal(t, "2026-06-10T09:00:00Z", resp["locked_since"])
}

func TestDeploy_MissingComponentSlug_Returns422(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(), successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{"tag": "v1.0.0"})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDeploy_MissingTag_Returns422(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(), successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{"component_slug": "my-service"})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDeploy_GitOpsError_Returns500(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(),
		&mockGitOpsApplier{applyFn: func(_ context.Context, _ gitops.ApplyParams) error {
			return errors.New("push failed")
		}},
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"component_slug": "my-service",
		"tag":            "v1.0.0",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeploy_OverlayNotFound_Returns422(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(),
		&mockGitOpsApplier{applyFn: func(_ context.Context, p gitops.ApplyParams) error {
			return &gitops.OverlayNotFoundError{Path: p.OverlayPath}
		}},
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"component_slug": "my-service",
		"tag":            "v1.0.0",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDeploy_EnvironmentNotFound_Returns404(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		&mockEnvironmentStore{}, // getByIDFn nil → ErrEnvironmentNotFound
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(), successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "unknown-env", map[string]string{
		"component_slug": "my-service",
		"tag":            "v1.0.0",
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeploy_ComponentNotFound_Returns404(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		&mockComponentStore{}, // getBySlugFn nil → ErrComponentNotFound
		acquiringLockStore(), successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"component_slug": "unknown-comp",
		"tag":            "v1.0.0",
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeploy_ViewerRole_Returns403(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		compStoreWithComp(deployFixtureComp),
		acquiringLockStore(), successApplier(),
		&domain.UserIdentity{
			Sub:          "user-2",
			Email:        "viewer@example.com",
			Name:         "Read Only",
			ProductRoles: map[string]domain.Role{"my-service": domain.RoleViewer},
		},
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"component_slug": "my-service",
		"tag":            "v1.0.0",
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
}
