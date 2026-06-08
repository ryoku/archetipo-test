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
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/store"
)

// --- mock Lister ---

type mockLister struct {
	listTagsFn func(ctx context.Context, imagePath, pageToken string, pageSize int) ([]gcr.Tag, string, error)
}

func (m *mockLister) ListTags(ctx context.Context, imagePath, pageToken string, pageSize int) ([]gcr.Tag, string, error) {
	if m.listTagsFn != nil {
		return m.listTagsFn(ctx, imagePath, pageToken, pageSize)
	}
	return nil, "", nil
}

var _ gcr.Lister = (*mockLister)(nil)

// --- router helper ---

func newTagRouter(ps store.ProductStore, cs store.ComponentStore, l gcr.Lister, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewTagHandlers(ps, cs, l)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	api.GET("/products/:productSlug/components/:componentSlug/tags",
		middleware.RequireRole(domain.RoleViewer),
		h.ListTags,
	)
	return r
}

// --- local fixtures ---

func tagProductStoreOK(slug string) store.ProductStore {
	p := makeProduct(slug)
	return &mockProductStore{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Product, error) { return &p, nil },
	}
}

func tagProductStoreNotFound() store.ProductStore {
	return &mockProductStore{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return nil, store.ErrNotFound
		},
	}
}

func tagCompStoreOK() store.ComponentStore {
	return &mockComponentStore{
		getBySlugFn: func(_ context.Context, _, _ string) (*domain.Component, error) {
			return &domain.Component{
				ID:           "comp-1",
				ProductID:    "prod-1",
				GCRImagePath: "us-docker.pkg.dev/proj/repo/img",
			}, nil
		},
	}
}

func tagCompStoreNotFound() store.ComponentStore {
	return &mockComponentStore{
		getBySlugFn: func(_ context.Context, _, _ string) (*domain.Component, error) {
			return nil, store.ErrComponentNotFound
		},
	}
}

// --- tests ---

func TestTagHandler_ListTags_OK(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return []gcr.Tag{
				{Name: "v1.0.0", Digest: "sha256:aaa", PushedAt: now},
				{Name: "latest", Digest: "sha256:aaa", PushedAt: now},
			}, "next-token", nil
		},
	}

	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Tags []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"tags"`
		NextPageToken string `json:"next_page_token"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(resp.Tags))
	}
	if resp.NextPageToken != "next-token" {
		t.Errorf("expected next_page_token=next-token, got %q", resp.NextPageToken)
	}
}

func TestTagHandler_ListTags_EmptyResult(t *testing.T) {
	lister := &mockLister{}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Tags []interface{} `json:"tags"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(resp.Tags))
	}
}

func TestTagHandler_ListTags_PageSizeForwarded(t *testing.T) {
	var got int
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _ string, pageSize int) ([]gcr.Tag, string, error) {
			got = pageSize
			return nil, "", nil
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags?page_size=5", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != 5 {
		t.Errorf("expected page_size=5 forwarded, got %d", got)
	}
}

func TestTagHandler_ListTags_InvalidPageSize(t *testing.T) {
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), &mockLister{}, viewerIdentity("my-product"))
	for _, qs := range []string{"page_size=0", "page_size=-1", "page_size=101", "page_size=abc"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags?"+qs, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", qs, w.Code)
		}
	}
}

func TestTagHandler_ListTags_ProductNotFound(t *testing.T) {
	r := newTagRouter(tagProductStoreNotFound(), tagCompStoreNotFound(), &mockLister{}, viewerIdentity("missing"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/missing/components/comp/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_ComponentNotFound(t *testing.T) {
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreNotFound(), &mockLister{}, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/missing/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRRateLimit(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: quota exceeded", gcr.ErrRateLimit)
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRAuthFailure(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: invalid credentials", gcr.ErrAuthFailure)
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRNetworkError(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: connection refused", gcr.ErrNetwork)
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_EmptyGCRImagePath(t *testing.T) {
	compStore := &mockComponentStore{
		getBySlugFn: func(_ context.Context, _, _ string) (*domain.Component, error) {
			return &domain.Component{ID: "comp-1", ProductID: "prod-1", GCRImagePath: ""}, nil
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), compStore, &mockLister{}, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/no-image/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for empty GCRImagePath, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRRepoNotFound(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: repository not found", gcr.ErrRepoNotFound)
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GCR ErrRepoNotFound, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_PageTokenForwarded(t *testing.T) {
	var gotToken string
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, pageToken string, _ int) ([]gcr.Tag, string, error) {
			gotToken = pageToken
			return nil, "", nil
		},
	}
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), lister, viewerIdentity("my-product"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags?page_token=abc123", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotToken != "abc123" {
		t.Errorf("expected page_token %q forwarded to lister, got %q", "abc123", gotToken)
	}
}

func TestTagHandler_ListTags_Unauthenticated(t *testing.T) {
	r := newTagRouter(tagProductStoreOK("my-product"), tagCompStoreOK(), &mockLister{}, nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/products/my-product/components/my-comp/tags", nil)
	r.ServeHTTP(w, req)
	// No identity → RequireRole middleware aborts with 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
