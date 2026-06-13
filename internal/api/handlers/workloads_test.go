package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWorkloadRouter(
	ps store.ProductStore,
	es store.EnvironmentStore,
	r gitops.WorkloadReader,
	identity *domain.UserIdentity,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	h := handlers.NewWorkloadHandlers(ps, es, r)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := eng.Group("/api/v1", injectIdentity)
	api.GET("/products/:productSlug/environments/:environmentID/workloads",
		middleware.RequireRole(domain.RoleViewer),
		h.ListWorkloads,
	)
	return eng
}

var wlFixtureProduct = &domain.Product{
	ID:   "prod-wl-1",
	Name: "Workload Product",
	Slug: "wl-product",
}

var wlFixtureEnv = &domain.Environment{
	ID:        "env-wl-1",
	ProductID: "prod-wl-1",
	Name:      "dev",
	Slug:      "dev",
	Type:      "dev",
}

func wlProductStore(p *domain.Product) store.ProductStore {
	return &mockProductStore{
		getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
			if slug == p.Slug {
				return p, nil
			}
			return nil, store.ErrNotFound
		},
	}
}

func wlEnvStore(e *domain.Environment) store.EnvironmentStore {
	return &mockEnvironmentStore{
		getByIDFn: func(_ context.Context, productID, envID string) (*domain.Environment, error) {
			if productID == e.ProductID && envID == e.ID {
				return e, nil
			}
			return nil, store.ErrEnvironmentNotFound
		},
	}
}

func doWorkloadRequest(t *testing.T, r *gin.Engine, productSlug, envID string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet,
		"/api/v1/products/"+productSlug+"/environments/"+envID+"/workloads", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListWorkloads_OK(t *testing.T) {
	reader := &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return []domain.Workload{
				{Name: "api", ImageRepository: "us-docker.pkg.dev/proj/repo/api"},
				{Name: "worker", ImageRepository: "us-docker.pkg.dev/proj/repo/worker"},
			}, nil
		},
	}
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		reader,
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []struct {
		Name            string `json:"name"`
		ImageRepository string `json:"image_repository"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, "api", resp[0].Name)
	assert.Equal(t, "us-docker.pkg.dev/proj/repo/api", resp[0].ImageRepository)
	assert.Equal(t, "worker", resp[1].Name)
}

func TestListWorkloads_EmptyHelmRelease_Returns200WithEmptyList(t *testing.T) {
	reader := &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return []domain.Workload{}, nil
		},
	}
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		reader,
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
}

func TestListWorkloads_HelmReleaseNotFound_Returns404(t *testing.T) {
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		workloadReaderHelmReleaseNotFound(),
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListWorkloads_HelmReleaseParseFailed_Returns422(t *testing.T) {
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		workloadReaderHelmReleaseParseFailed(),
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestListWorkloads_ReaderError_Returns500(t *testing.T) {
	reader := &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return nil, fmt.Errorf("unexpected git error")
		},
	}
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		reader,
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListWorkloads_GitOpsNotConfigured_Returns503(t *testing.T) {
	reader := &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return nil, gitops.ErrGitOpsNotConfigured
		},
	}
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		reader,
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestListWorkloads_ProductNotFound_Returns404(t *testing.T) {
	r := newWorkloadRouter(
		&mockProductStore{
			getBySlugFn: func(_ context.Context, _ string) (*domain.Product, error) {
				return nil, store.ErrNotFound
			},
		},
		wlEnvStore(wlFixtureEnv),
		workloadReaderOK(nil),
		viewerIdentity("missing"),
	)

	w := doWorkloadRequest(t, r, "missing", "env-wl-1")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListWorkloads_EnvironmentNotFound_Returns404(t *testing.T) {
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		&mockEnvironmentStore{
			getByIDFn: func(_ context.Context, _, _ string) (*domain.Environment, error) {
				return nil, store.ErrEnvironmentNotFound
			},
		},
		workloadReaderOK(nil),
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "missing-env")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListWorkloads_EnvStoreError_Returns500(t *testing.T) {
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		&mockEnvironmentStore{
			getByIDFn: func(_ context.Context, _, _ string) (*domain.Environment, error) {
				return nil, fmt.Errorf("db connection lost")
			},
		},
		workloadReaderOK(nil),
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListWorkloads_Unauthenticated_Returns401(t *testing.T) {
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		workloadReaderOK(nil),
		nil,
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListWorkloads_ReaderReceivesProductAndEnvSlugs(t *testing.T) {
	var capturedProduct, capturedEnv string
	reader := &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, productSlug, envSlug string) ([]domain.Workload, error) {
			capturedProduct = productSlug
			capturedEnv = envSlug
			return nil, nil
		},
	}
	r := newWorkloadRouter(
		wlProductStore(wlFixtureProduct),
		wlEnvStore(wlFixtureEnv),
		reader,
		viewerIdentity("wl-product"),
	)

	w := doWorkloadRequest(t, r, "wl-product", "env-wl-1")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "wl-product", capturedProduct)
	assert.Equal(t, "dev", capturedEnv)
}
