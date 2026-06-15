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

type mockAcquiredLock struct{}

func (m *mockAcquiredLock) Release(_ context.Context) error { return nil }

var _ store.AcquiredLock = (*mockAcquiredLock)(nil)

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

type mockGitOpsApplier struct {
	applyFn func(ctx context.Context, p gitops.ApplyParams) (string, error)
}

func (m *mockGitOpsApplier) Apply(ctx context.Context, p gitops.ApplyParams) (string, error) {
	if m.applyFn == nil {
		panic("mockGitOpsApplier.Apply called unexpectedly")
	}
	return m.applyFn(ctx, p)
}

var _ handlers.GitOpsApplier = (*mockGitOpsApplier)(nil)

type mockDeploymentStore struct {
	createFn  func(ctx context.Context, d *domain.Deployment) error
	getByIDFn func(ctx context.Context, id string) (*domain.Deployment, error)
}

func (m *mockDeploymentStore) Create(ctx context.Context, d *domain.Deployment) error {
	if m.createFn != nil {
		return m.createFn(ctx, d)
	}
	d.ID = "test-deployment-id"
	return nil
}
func (m *mockDeploymentStore) GetByID(ctx context.Context, id string) (*domain.Deployment, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, store.ErrDeploymentNotFound
}

var _ store.DeploymentStore = (*mockDeploymentStore)(nil)

func noopDeploymentStore() *mockDeploymentStore {
	return &mockDeploymentStore{}
}

func newDeployRouter(
	ps store.ProductStore,
	es store.EnvironmentStore,
	ls store.DeploymentLockStore,
	applier handlers.GitOpsApplier,
	identity *domain.UserIdentity,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(ps, es, ls, noopDeploymentStore(), applier, "")

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

var deployFixtureProduct = &domain.Product{
	ID:   "prod-id-1",
	Name: "My Service",
	Slug: "my-service",
}

var deployFixtureEnv = &domain.Environment{
	ID:        "env-id-1",
	ProductID: "prod-id-1",
	Name:      "dev",
	Slug:      "dev",
	Type:      "dev",
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
		applyFn: func(_ context.Context, _ gitops.ApplyParams) (string, error) { return "abc123", nil },
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

func TestDeploy_Success(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["deployment_id"])
}

func TestDeploy_Returns202WithDeploymentID(t *testing.T) {
	var storedCommitSHA string
	ds := &mockDeploymentStore{
		createFn: func(_ context.Context, d *domain.Deployment) error {
			storedCommitSHA = d.CommitSHA
			d.ID = "deploy-uuid-001"
			return nil
		},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		ds,
		&mockGitOpsApplier{applyFn: func(_ context.Context, _ gitops.ApplyParams) (string, error) {
			return "deadbeef", nil
		}},
		"",
	)
	injectIdentity := func(c *gin.Context) {
		c.Set(domain.IdentityContextKey, editorIdentityForDeploy("my-service"))
		c.Next()
	}
	api := r.Group("/api/v1", injectIdentity)
	api.POST("/products/:productSlug/environments/:environmentID/deployments",
		middleware.RequireRole(domain.RoleEditor), h.Deploy)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "deploy-uuid-001", resp["deployment_id"])
	assert.Equal(t, "deadbeef", storedCommitSHA)
}

func TestDeploy_GitOpsError_StoresFailureRecord(t *testing.T) {
	var storedOutcome string
	ds := &mockDeploymentStore{
		createFn: func(_ context.Context, d *domain.Deployment) error {
			storedOutcome = d.Outcome
			d.ID = "deploy-fail-id"
			return nil
		},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		ds,
		&mockGitOpsApplier{applyFn: func(_ context.Context, _ gitops.ApplyParams) (string, error) {
			return "", errors.New("push failed: auth error")
		}},
		"",
	)
	injectIdentity := func(c *gin.Context) {
		c.Set(domain.IdentityContextKey, editorIdentityForDeploy("my-service"))
		c.Next()
	}
	api := r.Group("/api/v1", injectIdentity)
	api.POST("/products/:productSlug/environments/:environmentID/deployments",
		middleware.RequireRole(domain.RoleEditor), h.Deploy)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, domain.OutcomeFailure, storedOutcome)
}

func TestDeploy_ApplyParamsUsesHelmReleasePath(t *testing.T) {
	var captured gitops.ApplyParams
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		&mockGitOpsApplier{applyFn: func(_ context.Context, p gitops.ApplyParams) (string, error) {
			captured = p
			return "sha123", nil
		}},
		editorIdentityForDeploy("my-service"),
	)

	doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v2.0.0",
	})

	assert.Equal(t, "apps/dev/my-service/my-service-helmrelease.yaml", captured.HelmReleasePath)
	assert.Equal(t, "main", captured.Workload)
	assert.Equal(t, "v2.0.0", captured.NewTag)
	assert.Equal(t, "my-service", captured.ProductSlug)
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
		heldLockStore(holder),
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "deployment in progress", resp["error"])
	assert.Equal(t, "Marco Tech Lead", resp["lock_holder"])
	assert.Equal(t, "2026-06-10T09:00:00Z", resp["locked_since"])
}

func TestDeploy_LockStoreError_Returns500(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		&mockDeploymentLockStore{
			tryAcquireFn: func(_ context.Context, _, _, _ string, _ time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
				return nil, nil, errors.New("db connection lost")
			},
		},
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeploy_MissingWorkload_Returns422(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
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
		acquiringLockStore(), successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{"workload": "main"})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDeploy_GitOpsError_Returns500(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		&mockGitOpsApplier{applyFn: func(_ context.Context, _ gitops.ApplyParams) (string, error) {
			return "", errors.New("push failed")
		}},
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeploy_WorkloadNotInHelmRelease_Returns422(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		&mockGitOpsApplier{applyFn: func(_ context.Context, p gitops.ApplyParams) (string, error) {
			return "", &gitops.HelmReleasePathError{Path: "spec.values." + p.Workload, Reason: "key not found"}
		}},
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "nonexistent",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "workload not found in HelmRelease")
}

func TestDeploy_HelmReleaseNotFound_Returns422(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		&mockGitOpsApplier{applyFn: func(_ context.Context, p gitops.ApplyParams) (string, error) {
			return "", &gitops.HelmReleaseNotFoundError{Path: p.HelmReleasePath}
		}},
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDeploy_EnvironmentNotFound_Returns404(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		&mockEnvironmentStore{},
		acquiringLockStore(), successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	w := doDeployRequest(t, r, "my-service", "unknown-env", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeploy_ViewerRole_Returns403(t *testing.T) {
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(), successApplier(),
		&domain.UserIdentity{
			Sub:          "user-2",
			Email:        "viewer@example.com",
			Name:         "Read Only",
			ProductRoles: map[string]domain.Role{"my-service": domain.RoleViewer},
		},
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeploymentLockTimeout_UsesEnvVar(t *testing.T) {
	t.Setenv("DEPLOYMENT_LOCK_TIMEOUT_SECONDS", "10")
	var capturedTimeout time.Duration
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		&mockDeploymentLockStore{
			tryAcquireFn: func(_ context.Context, _, _, _ string, timeout time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
				capturedTimeout = timeout
				return &mockAcquiredLock{}, nil, nil
			},
		},
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, 10*time.Second, capturedTimeout)
}

func TestDeploymentLockTimeout_UnsetDefaultsTo5s(t *testing.T) {
	t.Setenv("DEPLOYMENT_LOCK_TIMEOUT_SECONDS", "")
	var capturedTimeout time.Duration
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		&mockDeploymentLockStore{
			tryAcquireFn: func(_ context.Context, _, _, _ string, timeout time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
				capturedTimeout = timeout
				return &mockAcquiredLock{}, nil, nil
			},
		},
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, 5*time.Second, capturedTimeout)
}

func TestDeploymentLockTimeout_InvalidValueDefaultsTo5s(t *testing.T) {
	t.Setenv("DEPLOYMENT_LOCK_TIMEOUT_SECONDS", "banana")
	var capturedTimeout time.Duration
	r := newDeployRouter(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		&mockDeploymentLockStore{
			tryAcquireFn: func(_ context.Context, _, _, _ string, timeout time.Duration) (store.AcquiredLock, *domain.DeploymentLock, error) {
				capturedTimeout = timeout
				return &mockAcquiredLock{}, nil, nil
			},
		},
		successApplier(),
		editorIdentityForDeploy("my-service"),
	)

	doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "v1.0.0",
	})

	assert.Equal(t, 5*time.Second, capturedTimeout)
}

// --- Tag convention enforcement tests ---

const tagConventionTestDefault = `^v\d+\.\d+\.\d+$`

var deployFixtureProdEnv = &domain.Environment{
	ID:        "env-prod-1",
	ProductID: "prod-id-1",
	Name:      "production",
	Slug:      "production",
	Type:      "production",
}

func newDeployRouterWithConvention(
	ps store.ProductStore,
	es store.EnvironmentStore,
	ls store.DeploymentLockStore,
	applier handlers.GitOpsApplier,
	identity *domain.UserIdentity,
	defaultTagConvention string,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(ps, es, ls, noopDeploymentStore(), applier, defaultTagConvention)
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

func TestDeploy_ProductionEnv_NonConformingTag_Returns422(t *testing.T) {
	r := newDeployRouterWithConvention(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureProdEnv),
		&mockDeploymentLockStore{},
		&mockGitOpsApplier{},
		editorIdentityForDeploy("my-service"),
		tagConventionTestDefault,
	)

	w := doDeployRequest(t, r, "my-service", "env-prod-1", map[string]string{
		"workload": "main",
		"tag":      "latest",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "latest", resp["rejected_tag"])
	assert.Equal(t, tagConventionTestDefault, resp["applied_regex"])
	assert.NotEmpty(t, resp["message"])
}

func TestDeploy_ProductionEnv_ConformingTag_Returns202(t *testing.T) {
	r := newDeployRouterWithConvention(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureProdEnv),
		acquiringLockStore(),
		successApplier(),
		editorIdentityForDeploy("my-service"),
		tagConventionTestDefault,
	)

	w := doDeployRequest(t, r, "my-service", "env-prod-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestDeploy_DevEnv_NonConformingTag_NotValidated_Returns202(t *testing.T) {
	r := newDeployRouterWithConvention(
		productStoreWithProduct(deployFixtureProduct),
		envStoreWithEnv(deployFixtureEnv),
		acquiringLockStore(),
		successApplier(),
		editorIdentityForDeploy("my-service"),
		tagConventionTestDefault,
	)

	w := doDeployRequest(t, r, "my-service", "env-id-1", map[string]string{
		"workload": "main",
		"tag":      "latest",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestDeploy_ProductionEnv_ProductRegex_OverridesDefault_Returns422(t *testing.T) {
	productRegex := `^release-\d+$`
	productWithRegex := &domain.Product{
		ID:                 "prod-id-1",
		Name:               "My Service",
		Slug:               "my-service",
		TagConventionRegex: &productRegex,
	}
	r := newDeployRouterWithConvention(
		productStoreWithProduct(productWithRegex),
		envStoreWithEnv(deployFixtureProdEnv),
		&mockDeploymentLockStore{},
		&mockGitOpsApplier{},
		editorIdentityForDeploy("my-service"),
		tagConventionTestDefault,
	)

	w := doDeployRequest(t, r, "my-service", "env-prod-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, productRegex, resp["applied_regex"])
	assert.Equal(t, "v1.2.3", resp["rejected_tag"])
}

func TestDeploy_ProductionEnv_InvalidStoredRegex_Returns500(t *testing.T) {
	invalidRegex := `[invalid(`
	productWithBadRegex := &domain.Product{
		ID:                 "prod-id-1",
		Name:               "My Service",
		Slug:               "my-service",
		TagConventionRegex: &invalidRegex,
	}
	r := newDeployRouterWithConvention(
		productStoreWithProduct(productWithBadRegex),
		envStoreWithEnv(deployFixtureProdEnv),
		&mockDeploymentLockStore{},
		&mockGitOpsApplier{},
		editorIdentityForDeploy("my-service"),
		tagConventionTestDefault,
	)

	w := doDeployRequest(t, r, "my-service", "env-prod-1", map[string]string{
		"workload": "main",
		"tag":      "v1.2.3",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeploy_ProductionEnv_ProductRegex_ConformingTag_Returns202(t *testing.T) {
	productRegex := `^release-\d+$`
	productWithRegex := &domain.Product{
		ID:                 "prod-id-1",
		Name:               "My Service",
		Slug:               "my-service",
		TagConventionRegex: &productRegex,
	}
	r := newDeployRouterWithConvention(
		productStoreWithProduct(productWithRegex),
		envStoreWithEnv(deployFixtureProdEnv),
		acquiringLockStore(),
		successApplier(),
		editorIdentityForDeploy("my-service"),
		tagConventionTestDefault,
	)

	w := doDeployRequest(t, r, "my-service", "env-prod-1", map[string]string{
		"workload": "main",
		"tag":      "release-42",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
}
