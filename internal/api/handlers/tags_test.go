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
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// --- mock Lister ---

type mockLister struct {
	listTagsFn func(ctx context.Context, imagePath, pageToken, filter string, pageSize int) ([]gcr.Tag, string, error)
}

func (m *mockLister) ListTags(ctx context.Context, imagePath, pageToken, filter string, pageSize int) ([]gcr.Tag, string, error) {
	if m.listTagsFn != nil {
		return m.listTagsFn(ctx, imagePath, pageToken, filter, pageSize)
	}
	return nil, "", nil
}

var _ gcr.Lister = (*mockLister)(nil)

// --- mock WorkloadReader ---

type mockWorkloadReader struct {
	listWorkloadsFn func(ctx context.Context, productSlug, envSlug string) ([]domain.Workload, error)
}

func (m *mockWorkloadReader) ListWorkloads(ctx context.Context, productSlug, envSlug string) ([]domain.Workload, error) {
	if m.listWorkloadsFn != nil {
		return m.listWorkloadsFn(ctx, productSlug, envSlug)
	}
	return nil, nil
}

var _ gitops.WorkloadReader = (*mockWorkloadReader)(nil)

// --- router helper ---

func newTagRouter(
	ps store.ProductStore,
	es store.EnvironmentStore,
	r gitops.WorkloadReader,
	l gcr.Lister,
	identity *domain.UserIdentity,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	h := handlers.NewTagHandlers(ps, es, r, l)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := eng.Group("/api/v1", injectIdentity)
	api.GET("/products/:productSlug/environments/:environmentID/workloads/:workload/tags",
		middleware.RequireRole(domain.RoleViewer),
		h.ListTags,
	)
	return eng
}

// --- local fixtures ---

var tagFixtureProduct = &domain.Product{
	ID:   "prod-tag-1",
	Name: "Tag Product",
	Slug: "tag-product",
}

var tagFixtureEnv = &domain.Environment{
	ID:        "env-tag-1",
	ProductID: "prod-tag-1",
	Name:      "dev",
	Slug:      "dev",
	Type:      "dev",
}

var tagFixtureWorkloads = []domain.Workload{
	{Name: "main", ImageRepository: "us-docker.pkg.dev/proj/repo/img"},
	{Name: "sidecar", ImageRepository: "us-docker.pkg.dev/proj/repo/sidecar"},
}

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

func tagEnvStoreOK() store.EnvironmentStore {
	return &mockEnvironmentStore{
		getByIDFn: func(_ context.Context, _, _ string) (*domain.Environment, error) {
			return tagFixtureEnv, nil
		},
	}
}

func tagEnvStoreNotFound() store.EnvironmentStore {
	return &mockEnvironmentStore{
		getByIDFn: func(_ context.Context, _, _ string) (*domain.Environment, error) {
			return nil, store.ErrEnvironmentNotFound
		},
	}
}

func workloadReaderOK(workloads []domain.Workload) *mockWorkloadReader {
	return &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return workloads, nil
		},
	}
}

func workloadReaderHelmReleaseNotFound() *mockWorkloadReader {
	return &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return nil, gitops.ErrHelmReleaseNotFound
		},
	}
}

func workloadReaderHelmReleaseParseFailed() *mockWorkloadReader {
	return &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return nil, gitops.ErrHelmReleaseParseFailed
		},
	}
}

func doTagRequest(t *testing.T, r *gin.Engine, productSlug, envID, workload, query string) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/products/" + productSlug + "/environments/" + envID + "/workloads/" + workload + "/tags"
	if query != "" {
		url += "?" + query
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestTagHandler_ListTags_OK(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return []gcr.Tag{
				{Name: "v1.0.0", Digest: "sha256:aaa", PushedAt: now},
				{Name: "v1.1.0", Digest: "sha256:bbb", PushedAt: now},
			}, "next-token", nil
		},
	}

	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

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
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

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

func TestTagHandler_ListTags_WorkloadNotInHelmRelease_Returns404(t *testing.T) {
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "nonexistent", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown workload, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_HelmReleaseNotFound_Returns404(t *testing.T) {
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderHelmReleaseNotFound(),
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when HelmRelease not found, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_HelmReleaseParseFailed_Returns422(t *testing.T) {
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderHelmReleaseParseFailed(),
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 when HelmRelease parse fails, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_WorkloadReaderError_Returns500(t *testing.T) {
	reader := &mockWorkloadReader{
		listWorkloadsFn: func(_ context.Context, _, _ string) ([]domain.Workload, error) {
			return nil, fmt.Errorf("unexpected storage failure")
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		reader,
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for unexpected reader error, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_ProductNotFound(t *testing.T) {
	r := newTagRouter(
		tagProductStoreNotFound(),
		tagEnvStoreNotFound(),
		workloadReaderOK(nil),
		&mockLister{},
		viewerIdentity("missing"),
	)
	w := doTagRequest(t, r, "missing", "env-tag-1", "main", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_EnvironmentNotFound_Returns404(t *testing.T) {
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreNotFound(),
		workloadReaderOK(nil),
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "missing-env", "main", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when env not found, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_PageSizeForwarded(t *testing.T) {
	var got int
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, _ string, pageSize int) ([]gcr.Tag, string, error) {
			got = pageSize
			return nil, "", nil
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "page_size=5")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != 5 {
		t.Errorf("expected page_size=5 forwarded, got %d", got)
	}
}

func TestTagHandler_ListTags_InvalidPageSize(t *testing.T) {
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		&mockLister{},
		viewerIdentity("tag-product"),
	)
	for _, qs := range []string{"page_size=0", "page_size=-1", "page_size=101", "page_size=abc"} {
		w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", qs)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", qs, w.Code)
		}
	}
}

func TestTagHandler_ListTags_PageTokenForwarded(t *testing.T) {
	var gotToken string
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, pageToken, _ string, _ int) ([]gcr.Tag, string, error) {
			gotToken = pageToken
			return nil, "", nil
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "page_token=abc123")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotToken != "abc123" {
		t.Errorf("expected page_token %q forwarded, got %q", "abc123", gotToken)
	}
}

func TestTagHandler_ListTags_FilterForwarded(t *testing.T) {
	var gotFilter string
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, filter string, _ int) ([]gcr.Tag, string, error) {
			gotFilter = filter
			return nil, "", nil
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "filter=v1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotFilter != "v1" {
		t.Errorf("expected filter %q forwarded, got %q", "v1", gotFilter)
	}
}

func TestTagHandler_ListTags_GCRRateLimit(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: quota exceeded", gcr.ErrRateLimit)
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRAuthFailure(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: invalid credentials", gcr.ErrAuthFailure)
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRNetworkError(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: connection refused", gcr.ErrNetwork)
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_GCRRepoNotFound(t *testing.T) {
	lister := &mockLister{
		listTagsFn: func(_ context.Context, _, _, _ string, _ int) ([]gcr.Tag, string, error) {
			return nil, "", fmt.Errorf("%w: repository not found", gcr.ErrRepoNotFound)
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GCR ErrRepoNotFound, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_Unauthenticated(t *testing.T) {
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		&mockLister{},
		nil,
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTagHandler_ListTags_ImageRepoPassedToLister(t *testing.T) {
	var gotImageRepo string
	lister := &mockLister{
		listTagsFn: func(_ context.Context, imagePath, _, _ string, _ int) ([]gcr.Tag, string, error) {
			gotImageRepo = imagePath
			return nil, "", nil
		},
	}
	r := newTagRouter(
		tagProductStoreOK("tag-product"),
		tagEnvStoreOK(),
		workloadReaderOK(tagFixtureWorkloads),
		lister,
		viewerIdentity("tag-product"),
	)
	w := doTagRequest(t, r, "tag-product", "env-tag-1", "main", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotImageRepo != "us-docker.pkg.dev/proj/repo/img" {
		t.Errorf("expected image repo from workload discovery, got %q", gotImageRepo)
	}
}
