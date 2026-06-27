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
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeploymentStore is defined in deploy_test.go (shared across this package).

func newHistoryRouter(ps store.ProductStore, ds store.DeploymentStore, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewHistoryHandlers(ps, ds)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	api.GET("/products/:productSlug/deployments", middleware.RequireRole(domain.RoleViewer), h.ListByProduct)
	api.GET("/admin/deployments", middleware.RequireAdmin(), h.ListAll)
	return r
}

func fixedDeployments(n int) []domain.Deployment {
	result := make([]domain.Deployment, n)
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	for i := range n {
		sha := "sha" + string(rune('0'+i))
		result[i] = domain.Deployment{
			ID:               "id-" + string(rune('a'+i)),
			ProductID:        "prod-1",
			EnvironmentID:    "env-1",
			ActorDisplayName: "Marco Andreoli",
			ComponentName:    "api",
			EnvironmentName:  "production",
			Tag:              "v1.0." + string(rune('0'+i)),
			DeployedAt:       base.Add(time.Duration(i) * time.Hour),
			CommitSHA:        &sha,
			Outcome:          domain.OutcomeSuccess,
		}
	}
	return result
}

// --- tests for ListByProduct ---

func TestHistoryHandler_ListByProduct_OK(t *testing.T) {
	deps := fixedDeployments(3)
	ps := &mockProductStore{
		getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
			p := makeProduct(slug)
			return &p, nil
		},
	}
	ds := &mockDeploymentStore{
		listByProductFn: func(_ context.Context, _ string, page, _ int) ([]domain.Deployment, int, error) {
			return deps, 3, nil
		},
	}

	r := newHistoryRouter(ps, ds, viewerIdentity("test-product"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/test-product/deployments", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(3), resp["total"])
	assert.Equal(t, float64(1), resp["page"])
	assert.Len(t, resp["deployments"], 3)
}

func TestHistoryHandler_ListByProduct_PageParam(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		wantPage int
	}{
		{"valid page param", "?page=3", 3},
		{"invalid page defaults to 1", "?page=0", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps := &mockProductStore{
				getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
					p := makeProduct(slug)
					return &p, nil
				},
			}
			var capturedPage int
			ds := &mockDeploymentStore{
				listByProductFn: func(_ context.Context, _ string, page, _ int) ([]domain.Deployment, int, error) {
					capturedPage = page
					return nil, 0, nil
				},
			}

			r := newHistoryRouter(ps, ds, viewerIdentity("test-product"))
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/products/test-product/deployments"+tc.query, nil)
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tc.wantPage, capturedPage)
		})
	}
}

func TestHistoryHandler_ListByProduct_ProductNotFound(t *testing.T) {
	ps := &mockProductStore{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return nil, store.ErrNotFound
		},
	}
	ds := &mockDeploymentStore{}

	r := newHistoryRouter(ps, ds, viewerIdentity("missing"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/missing/deployments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHistoryHandler_ListByProduct_RequiresViewer(t *testing.T) {
	ps := &mockProductStore{}
	ds := &mockDeploymentStore{}

	r := newHistoryRouter(ps, ds, nil) // no identity → 401 unauthenticated
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/test-product/deployments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- tests for ListAll ---

func TestHistoryHandler_ListAll_OK(t *testing.T) {
	deps := fixedDeployments(5)
	ds := &mockDeploymentStore{
		listAllFn: func(_ context.Context, _, _ int) ([]domain.Deployment, int, error) {
			return deps, 5, nil
		},
	}

	r := newHistoryRouter(nil, ds, adminIdentity())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/deployments", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(5), resp["total"])
	assert.Len(t, resp["deployments"], 5)
}

func TestHistoryHandler_ListAll_RequiresAdmin(t *testing.T) {
	ds := &mockDeploymentStore{}

	r := newHistoryRouter(nil, ds, viewerIdentity("test-product"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/deployments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHistoryHandler_ListByProduct_StoreError(t *testing.T) {
	ps := &mockProductStore{
		getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
			p := makeProduct(slug)
			return &p, nil
		},
	}
	ds := &mockDeploymentStore{
		listByProductFn: func(_ context.Context, _ string, _, _ int) ([]domain.Deployment, int, error) {
			return nil, 0, fmt.Errorf("db unavailable")
		},
	}

	r := newHistoryRouter(ps, ds, viewerIdentity("test-product"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/test-product/deployments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHistoryHandler_ListAll_StoreError(t *testing.T) {
	ds := &mockDeploymentStore{
		listAllFn: func(_ context.Context, _, _ int) ([]domain.Deployment, int, error) {
			return nil, 0, fmt.Errorf("db unavailable")
		},
	}

	r := newHistoryRouter(nil, ds, adminIdentity())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/deployments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
