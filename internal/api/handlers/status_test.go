package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
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

// mockStatusReader is a test double for handlers.StatusReader.
type mockStatusReader struct {
	readCurrentTagsFn func(ctx context.Context, productSlug, envSlug string) (map[string]string, error)
}

func (m *mockStatusReader) ReadCurrentTags(ctx context.Context, productSlug, envSlug string) (map[string]string, error) {
	if m.readCurrentTagsFn != nil {
		return m.readCurrentTagsFn(ctx, productSlug, envSlug)
	}
	return nil, nil
}

func newStatusRouter(
	ps store.ProductStore,
	es store.EnvironmentStore,
	r handlers.StatusReader,
	identity *domain.UserIdentity,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	h := handlers.NewStatusHandlers(ps, es, r)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := eng.Group("/api/v1", injectIdentity)
	api.GET("/products/:productSlug/status",
		middleware.RequireRole(domain.RoleViewer),
		h.GetStatus,
	)
	return eng
}

var statusFixtureProduct = &domain.Product{
	ID:   "prod-status-1",
	Name: "Status Product",
	Slug: "status-product",
}

var statusFixtureEnvDev = domain.Environment{
	ID:        "env-status-dev",
	ProductID: "prod-status-1",
	Name:      "dev",
	Slug:      "dev",
	Type:      "dev",
}

var statusFixtureEnvProd = domain.Environment{
	ID:        "env-status-prod",
	ProductID: "prod-status-1",
	Name:      "production",
	Slug:      "production",
	Type:      "production",
}

func statusProductStore(p *domain.Product) store.ProductStore {
	return &mockProductStore{
		getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
			if slug == p.Slug {
				return p, nil
			}
			return nil, store.ErrNotFound
		},
	}
}

func statusEnvStore(envs ...domain.Environment) store.EnvironmentStore {
	return &mockEnvironmentStore{
		listByProductFn: func(_ context.Context, productID string) ([]domain.Environment, error) {
			var result []domain.Environment
			for _, e := range envs {
				if e.ProductID == productID {
					result = append(result, e)
				}
			}
			return result, nil
		},
	}
}

func doStatusRequest(t *testing.T, r *gin.Engine, productSlug string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/"+productSlug+"/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetStatus_OK(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, envSlug string) (map[string]string, error) {
			switch envSlug {
			case "dev":
				return map[string]string{"api": "v1.2.3", "worker": "v1.0.0"}, nil
			case "production":
				return map[string]string{"api": "v1.0.0", "worker": "v0.9.0"}, nil
			}
			return nil, nil
		},
	}
	identity := statusViewerIdentity(statusFixtureProduct.Slug)
	r := newStatusRouter(
		statusProductStore(statusFixtureProduct),
		statusEnvStore(statusFixtureEnvDev, statusFixtureEnvProd),
		reader,
		identity,
	)
	w := doStatusRequest(t, r, statusFixtureProduct.Slug)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	workloads, ok := resp["workloads"].(map[string]interface{})
	require.True(t, ok)
	api, ok := workloads["api"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "v1.2.3", api["dev"])
	assert.Equal(t, "v1.0.0", api["production"])
	assert.False(t, resp["stale"].(bool))
	assert.NotEmpty(t, resp["fetched_at"])
}

func TestGetStatus_ProductNotFound(t *testing.T) {
	reader := &mockStatusReader{}
	identity := &domain.UserIdentity{IsDevOpsAdmin: true}
	ps := &mockProductStore{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return nil, store.ErrNotFound
		},
	}
	r := newStatusRouter(ps, statusEnvStore(), reader, identity)
	w := doStatusRequest(t, r, "nonexistent")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetStatus_Unauthorized(t *testing.T) {
	reader := &mockStatusReader{}
	r := newStatusRouter(statusProductStore(statusFixtureProduct), statusEnvStore(), reader, nil)
	w := doStatusRequest(t, r, statusFixtureProduct.Slug)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetStatus_NoRole(t *testing.T) {
	reader := &mockStatusReader{}
	identity := &domain.UserIdentity{
		Sub:          "user-no-role",
		ProductRoles: map[string]string{},
	}
	r := newStatusRouter(statusProductStore(statusFixtureProduct), statusEnvStore(), reader, identity)
	w := doStatusRequest(t, r, statusFixtureProduct.Slug)
	// anti-enumeration: returns 404 for users with no role
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetStatus_HelmReleaseNotFound_ReturnsEmptyWorkloads(t *testing.T) {
	// When HelmRelease is not found for an env, that env is skipped (N/A for that env's workloads).
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, &gitops.HelmReleaseNotFoundError{Path: "apps/dev/p/p-helmrelease.yaml"}
		},
	}
	identity := statusViewerIdentity(statusFixtureProduct.Slug)
	r := newStatusRouter(
		statusProductStore(statusFixtureProduct),
		statusEnvStore(statusFixtureEnvDev),
		reader,
		identity,
	)
	w := doStatusRequest(t, r, statusFixtureProduct.Slug)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	workloads := resp["workloads"].(map[string]interface{})
	assert.Empty(t, workloads)
}

func TestGetStatus_GitOpsNotConfigured(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, fmt.Errorf("%w: not configured", gitops.ErrGitOpsNotConfigured)
		},
	}
	identity := statusViewerIdentity(statusFixtureProduct.Slug)
	r := newStatusRouter(
		statusProductStore(statusFixtureProduct),
		statusEnvStore(statusFixtureEnvDev),
		reader,
		identity,
	)
	w := doStatusRequest(t, r, statusFixtureProduct.Slug)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetStatus_CacheHit_ReaderCalledOnce(t *testing.T) {
	callCount := 0
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			callCount++
			return map[string]string{"api": "v1.0.0"}, nil
		},
	}
	identity := statusViewerIdentity(statusFixtureProduct.Slug)
	r := newStatusRouter(
		statusProductStore(statusFixtureProduct),
		statusEnvStore(statusFixtureEnvDev),
		reader,
		identity,
	)
	doStatusRequest(t, r, statusFixtureProduct.Slug)
	doStatusRequest(t, r, statusFixtureProduct.Slug)
	// Both requests share the same handler (and thus the same cache), so reader is called once.
	assert.Equal(t, 1, callCount)
}

// statusViewerIdentity creates a UserIdentity with the viewer role on the given product slug.
func statusViewerIdentity(slug string) *domain.UserIdentity {
	return &domain.UserIdentity{
		Sub:          "user-status-viewer",
		ProductRoles: map[string]string{slug: domain.RoleViewer},
	}
}

// Ensure mockStatusReader implements handlers.StatusReader at compile time.
var _ handlers.StatusReader = (*mockStatusReader)(nil)

// Ensure time import is used (for future stale tests).
var _ = time.Second
