package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

const shortTTL = 50 * time.Millisecond

type mockStatusReader struct {
	readCurrentTagsFn func(ctx context.Context, productSlug, envSlug string) (map[string]string, error)
	callCount         int
}

func (m *mockStatusReader) ReadCurrentTags(ctx context.Context, productSlug, envSlug string) (map[string]string, error) {
	m.callCount++
	return m.readCurrentTagsFn(ctx, productSlug, envSlug)
}

func newStatusRouter(
	ps store.ProductStore,
	es store.EnvironmentStore,
	r gitops.StatusReader,
	identity *domain.UserIdentity,
) *gin.Engine {
	return newStatusRouterWithTTL(ps, es, r, identity, 60*time.Second)
}

func newStatusRouterWithTTL(
	ps store.ProductStore,
	es store.EnvironmentStore,
	r gitops.StatusReader,
	identity *domain.UserIdentity,
	ttl time.Duration,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	h := handlers.NewStatusHandlersWithTTL(ps, es, r, ttl)

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

func mustGetStatus(t *testing.T, r *gin.Engine) handlers.StatusResponse {
	t.Helper()
	w := doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	assertStatus(t, w, http.StatusOK)
	var resp handlers.StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal status response: %v", err)
	}
	return resp
}

var statusFixtureProduct = &domain.Product{
	ID:   "prod-status-1",
	Name: "Status Product",
	Slug: "status-product",
}

var statusFixtureEnvDev = domain.Environment{
	ID:        "env-dev-1",
	ProductID: "prod-status-1",
	Name:      "dev",
	Slug:      "dev",
	Type:      "dev",
}

var statusFixtureEnvProd = domain.Environment{
	ID:        "env-prod-1",
	ProductID: "prod-status-1",
	Name:      "production",
	Slug:      "production",
	Type:      "production",
}

func statusProductStore() store.ProductStore {
	return &mockProductStore{
		getBySlugFn: func(_ context.Context, slug string) (*domain.Product, error) {
			if slug == statusFixtureProduct.Slug {
				return statusFixtureProduct, nil
			}
			return nil, store.ErrNotFound
		},
	}
}

func statusEnvStore(envs []domain.Environment) store.EnvironmentStore {
	return &mockEnvironmentStore{
		listByProductFn: func(_ context.Context, productID string) ([]domain.Environment, error) {
			if productID == statusFixtureProduct.ID {
				return envs, nil
			}
			return nil, fmt.Errorf("product not found")
		},
	}
}

func TestGetStatus_HappyPath(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, envSlug string) (map[string]string, error) {
			if envSlug == "dev" {
				return map[string]string{"api": "v1.2.0", "worker": "v1.1.0"}, nil
			}
			return map[string]string{"api": "v1.1.0", "worker": "v1.0.0"}, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev, statusFixtureEnvProd}), reader, viewerIdentity("status-product"))

	resp := mustGetStatus(t, r)
	if resp.Stale {
		t.Error("expected stale=false on fresh response")
	}
	if resp.FetchedAt == "" {
		t.Error("expected fetched_at to be set")
	}
	if resp.Workloads["api"]["dev"] != "v1.2.0" {
		t.Errorf("api/dev = %q, want %q", resp.Workloads["api"]["dev"], "v1.2.0")
	}
	if resp.Workloads["api"]["production"] != "v1.1.0" {
		t.Errorf("api/production = %q, want %q", resp.Workloads["api"]["production"], "v1.1.0")
	}
}

func TestGetStatus_Unauthenticated_Returns401(t *testing.T) {
	// No identity in context (no JWT) — RequireRole returns 401 before the handler runs.
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore(nil), reader, nil)

	w := doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestGetStatus_CacheHit_ReaderCalledOnce(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, envSlug string) (map[string]string, error) {
			return map[string]string{"api": "v1.0.0"}, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader, viewerIdentity("status-product"))

	doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")

	// reader should be called once (for the single env) on the first request; second hits cache
	if reader.callCount != 1 {
		t.Errorf("reader.callCount = %d, want 1 (cache should prevent second call)", reader.callCount)
	}
}

func TestGetStatus_StaleCache_ReturnsStaleTrueAndEvicts(t *testing.T) {
	callCount := 0
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			callCount++
			return map[string]string{"api": "v1.0.0"}, nil
		},
	}
	r := newStatusRouterWithTTL(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader, viewerIdentity("status-product"), shortTTL)

	// first request: no cache → fresh fetch
	resp1 := mustGetStatus(t, r)
	if resp1.Stale {
		t.Error("first response should not be stale")
	}

	time.Sleep(shortTTL + 10*time.Millisecond)

	// second request: cache exists but TTL expired → stale; returns stale=true and evicts
	resp2 := mustGetStatus(t, r)
	if !resp2.Stale {
		t.Error("second response should be stale when TTL=0")
	}

	// third request: cache was evicted → fresh fetch again
	resp3 := mustGetStatus(t, r)
	if resp3.Stale {
		t.Error("third response should not be stale (re-fetched after eviction)")
	}
	if callCount != 2 {
		t.Errorf("reader called %d times, want 2 (fresh fetch on req1 and req3)", callCount)
	}
}

func TestGetStatus_StaleCache_PreservesFetchedAt(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return map[string]string{"api": "v1.0.0"}, nil
		},
	}
	r := newStatusRouterWithTTL(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader, viewerIdentity("status-product"), shortTTL)

	resp1 := mustGetStatus(t, r)

	time.Sleep(shortTTL + 10*time.Millisecond)

	resp2 := mustGetStatus(t, r)
	if !resp2.Stale {
		t.Error("expected stale=true on second response after TTL expiry")
	}
	if resp2.FetchedAt != resp1.FetchedAt {
		t.Errorf("stale response fetched_at = %q, want %q (original value)", resp2.FetchedAt, resp1.FetchedAt)
	}
}

func TestGetStatus_HelmReleaseNotFound_SkipsEnv(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, envSlug string) (map[string]string, error) {
			if envSlug == "dev" {
				return map[string]string{"api": "v1.0.0"}, nil
			}
			return nil, &gitops.HelmReleaseNotFoundError{Path: "apps/production/status-product/status-product-helmrelease.yaml"}
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev, statusFixtureEnvProd}), reader, viewerIdentity("status-product"))

	resp := mustGetStatus(t, r)
	// dev tag present
	if resp.Workloads["api"]["dev"] != "v1.0.0" {
		t.Errorf("api/dev = %q, want %q", resp.Workloads["api"]["dev"], "v1.0.0")
	}
	// production env had no HelmRelease: api/production should be N/A (filled by gap logic)
	if resp.Workloads["api"]["production"] != "N/A" {
		t.Errorf("api/production = %q, want %q", resp.Workloads["api"]["production"], "N/A")
	}
}

func TestGetStatus_ProductNotFound_Returns404(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore(nil), reader, viewerIdentity("missing-product"))

	w := doPlain(r, http.MethodGet, "/api/v1/products/missing-product/status")
	assertStatus(t, w, http.StatusNotFound)
}

func TestGetStatus_NoRoleUser_Returns404(t *testing.T) {
	// RequireRole returns 404 (not 403) for users with no product role — anti-enumeration design.
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore(nil), reader, noRoleIdentity())

	w := doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	assertStatus(t, w, http.StatusNotFound)
}

func TestGetStatus_GitOpsNotConfigured_Returns503(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, fmt.Errorf("%w: set GITOPS_REPO_URL", gitops.ErrGitOpsNotConfigured)
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader, viewerIdentity("status-product"))

	w := doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	assertStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetStatus_FetchedAtIsRFC3339(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return map[string]string{"api": "v1.0.0"}, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader, viewerIdentity("status-product"))

	resp := mustGetStatus(t, r)
	if _, err := time.Parse(time.RFC3339, resp.FetchedAt); err != nil {
		t.Errorf("fetched_at %q is not valid RFC3339: %v", resp.FetchedAt, err)
	}
}

func TestGetStatus_InvalidSlug_Returns400(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, nil
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore(nil), reader, viewerIdentity("INVALID"))

	w := doPlain(r, http.MethodGet, "/api/v1/products/INVALID/status")
	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetStatus_EnvStoreFails_Returns500(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, nil
		},
	}
	brokenEnvStore := &mockEnvironmentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Environment, error) {
			return nil, fmt.Errorf("db connection lost")
		},
	}
	r := newStatusRouter(statusProductStore(), brokenEnvStore, reader, viewerIdentity("status-product"))

	w := doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestGetStatus_GenericReaderError_Returns500(t *testing.T) {
	reader := &mockStatusReader{
		readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
			return nil, fmt.Errorf("unexpected gitops error")
		},
	}
	r := newStatusRouter(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader, viewerIdentity("status-product"))

	w := doPlain(r, http.MethodGet, "/api/v1/products/status-product/status")
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestGetStatus_NewStatusHandlers_ReadsEnvTTL(t *testing.T) {
	t.Run("valid TTL from env", func(t *testing.T) {
		t.Setenv("STATUS_CACHE_TTL_SECONDS", "30")
		reader := &mockStatusReader{
			readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
				return map[string]string{"api": "v1.0.0"}, nil
			},
		}
		h := handlers.NewStatusHandlers(statusProductStore(), statusEnvStore([]domain.Environment{statusFixtureEnvDev}), reader)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
	})
	t.Run("invalid TTL defaults", func(t *testing.T) {
		t.Setenv("STATUS_CACHE_TTL_SECONDS", "not-a-number")
		reader := &mockStatusReader{
			readCurrentTagsFn: func(_ context.Context, _, _ string) (map[string]string, error) {
				return nil, nil
			},
		}
		h := handlers.NewStatusHandlers(statusProductStore(), statusEnvStore(nil), reader)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
	})
}
