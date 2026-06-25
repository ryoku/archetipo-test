package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStatsStore is a test double for store.StatsStore.
type mockStatsStore struct {
	getStatsFn func(ctx context.Context, slugs []string, isAdmin bool) (domain.Stats, error)
	callCount  int
}

func (m *mockStatsStore) GetStats(ctx context.Context, slugs []string, isAdmin bool) (domain.Stats, error) {
	m.callCount++
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx, slugs, isAdmin)
	}
	return domain.Stats{}, nil
}

var _ store.StatsStore = (*mockStatsStore)(nil)

func newStatsRouter(s store.StatsStore, identity *domain.UserIdentity, ttl time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewStatsHandlersWithTTL(s, ttl)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	api.GET("/stats", h.GetStats)
	return r
}

func TestStatsHandler_GetStats_OK(t *testing.T) {
	want := domain.Stats{ProductCount: 3, EnvironmentCount: 7, ComponentCount: 5, DeploymentsToday: 12}
	ms := &mockStatsStore{
		getStatsFn: func(_ context.Context, _ []string, _ bool) (domain.Stats, error) {
			return want, nil
		},
	}
	identity := &domain.UserIdentity{ProductRoles: map[string]domain.Role{"p1": domain.RoleViewer}}
	r := newStatsRouter(ms, identity, time.Minute)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got domain.Stats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, want, got)
}

func TestStatsHandler_CacheHit_SkipsSecondStoreCall(t *testing.T) {
	ms := &mockStatsStore{
		getStatsFn: func(_ context.Context, _ []string, _ bool) (domain.Stats, error) {
			return domain.Stats{ProductCount: 1}, nil
		},
	}
	identity := &domain.UserIdentity{ProductRoles: map[string]domain.Role{"p1": domain.RoleViewer}}
	// Long TTL so first response is definitely cached when the second request arrives.
	r := newStatsRouter(ms, identity, time.Minute)

	for i := range 2 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "request %d", i)
	}

	assert.Equal(t, 1, ms.callCount, "store should be called only once; second hit served from cache")
}

func TestStatsHandler_CacheExpiry_HitsStoreAgain(t *testing.T) {
	ms := &mockStatsStore{
		getStatsFn: func(_ context.Context, _ []string, _ bool) (domain.Stats, error) {
			return domain.Stats{ProductCount: 1}, nil
		},
	}
	identity := &domain.UserIdentity{ProductRoles: map[string]domain.Role{"p1": domain.RoleViewer}}
	// Zero TTL: every request is a cache miss.
	r := newStatsRouter(ms, identity, 0)

	for i := range 2 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "request %d", i)
	}

	assert.Equal(t, 2, ms.callCount, "store should be called twice when TTL is zero")
}

func TestStatsHandler_AdminIdentity_PassesIsAdminTrue(t *testing.T) {
	var capturedIsAdmin bool
	ms := &mockStatsStore{
		getStatsFn: func(_ context.Context, _ []string, isAdmin bool) (domain.Stats, error) {
			capturedIsAdmin = isAdmin
			return domain.Stats{}, nil
		},
	}
	identity := &domain.UserIdentity{IsDevOpsAdmin: true}
	r := newStatsRouter(ms, identity, 0)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedIsAdmin)
}

func TestStatsHandler_NonAdminIdentity_PassesCorrectSlugs(t *testing.T) {
	var capturedSlugs []string
	ms := &mockStatsStore{
		getStatsFn: func(_ context.Context, slugs []string, _ bool) (domain.Stats, error) {
			capturedSlugs = slugs
			return domain.Stats{}, nil
		},
	}
	identity := &domain.UserIdentity{
		ProductRoles: map[string]domain.Role{
			"alpha": domain.RoleEditor,
			"beta":  domain.RoleViewer,
		},
	}
	r := newStatsRouter(ms, identity, 0)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.ElementsMatch(t, []string{"alpha", "beta"}, capturedSlugs)
}

func TestStatsHandler_StoreError_Returns500(t *testing.T) {
	ms := &mockStatsStore{
		getStatsFn: func(_ context.Context, _ []string, _ bool) (domain.Stats, error) {
			return domain.Stats{}, errors.New("db is down")
		},
	}
	identity := &domain.UserIdentity{ProductRoles: map[string]domain.Role{"p1": domain.RoleViewer}}
	r := newStatsRouter(ms, identity, 0)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStatsHandler_NoIdentity_Returns401(t *testing.T) {
	ms := &mockStatsStore{}
	r := newStatsRouter(ms, nil, 0)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
