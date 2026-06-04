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
	"github.com/ryoku/kubegate/internal/store"
)

// --- mock store ---

type mockProductStore struct {
	createFn    func(ctx context.Context, p *domain.Product) error
	listFn      func(ctx context.Context, opts store.ListOptions) ([]domain.Product, error)
	getBySlugFn func(ctx context.Context, slug string) (*domain.Product, error)
	updateFn    func(ctx context.Context, slug, name, description string) (*domain.Product, error)
	archiveFn   func(ctx context.Context, slug string) error
}

func (m *mockProductStore) Create(ctx context.Context, p *domain.Product) error {
	return m.createFn(ctx, p)
}
func (m *mockProductStore) List(ctx context.Context, opts store.ListOptions) ([]domain.Product, error) {
	return m.listFn(ctx, opts)
}
func (m *mockProductStore) GetBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	return m.getBySlugFn(ctx, slug)
}
func (m *mockProductStore) Update(ctx context.Context, slug, name, description string) (*domain.Product, error) {
	return m.updateFn(ctx, slug, name, description)
}
func (m *mockProductStore) Archive(ctx context.Context, slug string) error {
	return m.archiveFn(ctx, slug)
}

var _ store.ProductStore = (*mockProductStore)(nil)

// --- helpers ---

func newRouter(s store.ProductStore, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewProductHandlers(s)

	// Inject identity via context setter — simulates JWTAuth having already run
	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	api.POST("/products", middleware.RequireAdmin(), h.CreateProduct)
	api.GET("/products", h.ListProducts)
	api.PUT("/products/:productSlug", middleware.RequireRole(domain.RoleEditor), h.UpdateProduct)
	api.DELETE("/products/:productSlug", middleware.RequireRole(domain.RoleEditor), h.ArchiveProduct)
	return r
}

func adminIdentity() *domain.UserIdentity {
	return &domain.UserIdentity{Sub: "admin-1", IsDevOpsAdmin: true}
}

func editorIdentity(slug string) *domain.UserIdentity {
	return &domain.UserIdentity{
		Sub:          "editor-1",
		ProductRoles: map[string]domain.Role{slug: domain.RoleEditor},
	}
}

func viewerIdentity(slug string) *domain.UserIdentity {
	return &domain.UserIdentity{
		Sub:          "viewer-1",
		ProductRoles: map[string]domain.Role{slug: domain.RoleViewer},
	}
}

func noRoleIdentity() *domain.UserIdentity {
	return &domain.UserIdentity{Sub: "norole-1", ProductRoles: map[string]domain.Role{}}
}

func now() time.Time { return time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC) }

func makeProduct(slug string) domain.Product {
	return domain.Product{
		ID:        "id-" + slug,
		Name:      "Product " + slug,
		Slug:      slug,
		CreatedAt: now(),
	}
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// --- CreateProduct tests ---

func TestCreateProduct_AdminCreatesProduct(t *testing.T) {
	s := &mockProductStore{
		createFn: func(_ context.Context, p *domain.Product) error {
			p.ID = "new-id"
			p.CreatedAt = now()
			return nil
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "My Product", "slug": "my-product", "description": "desc"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateProduct_InvalidSlug_Returns422(t *testing.T) {
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "Bad Slug", "slug": "Bad Slug!"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestCreateProduct_SlugConflict_Returns409(t *testing.T) {
	s := &mockProductStore{
		createFn: func(_ context.Context, _ *domain.Product) error {
			return store.ErrSlugConflict
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "Dupe", "slug": "existing-slug"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateProduct_NonAdminForbidden(t *testing.T) {
	r := newRouter(&mockProductStore{}, editorIdentity("some-product"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "X", "slug": "x"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateProduct_MissingName_Returns422(t *testing.T) {
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"slug": "valid-slug"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

// --- ListProducts tests ---

func TestListProducts_AdminReceivesAll(t *testing.T) {
	allProducts := []domain.Product{makeProduct("alpha"), makeProduct("beta")}
	s := &mockProductStore{
		listFn: func(_ context.Context, opts store.ListOptions) ([]domain.Product, error) {
			if opts.SlugAllowlist != nil {
				t.Error("admin path should not set SlugAllowlist")
			}
			return allProducts, nil
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 products, got %d", len(resp))
	}
}

func TestListProducts_UserReceivesOnlyOwnProducts(t *testing.T) {
	s := &mockProductStore{
		listFn: func(_ context.Context, opts store.ListOptions) ([]domain.Product, error) {
			if opts.SlugAllowlist == nil {
				t.Error("non-admin should have SlugAllowlist set")
			}
			if _, ok := opts.SlugAllowlist["my-product"]; !ok {
				t.Error("allowlist should contain my-product")
			}
			return []domain.Product{makeProduct("my-product")}, nil
		},
	}
	r := newRouter(s, editorIdentity("my-product"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListProducts_IncludeArchivedParam(t *testing.T) {
	s := &mockProductStore{
		listFn: func(_ context.Context, opts store.ListOptions) ([]domain.Product, error) {
			if !opts.IncludeArchived {
				t.Error("expected IncludeArchived=true from query param")
			}
			return []domain.Product{}, nil
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?include_archived=true", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListProducts_NoRoleUserReceivesEmptyList(t *testing.T) {
	s := &mockProductStore{
		listFn: func(_ context.Context, opts store.ListOptions) ([]domain.Product, error) {
			return []domain.Product{}, nil
		},
	}
	r := newRouter(s, noRoleIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

// --- UpdateProduct tests ---

func TestUpdateProduct_EditorUpdatesOwnProduct(t *testing.T) {
	updated := makeProduct("my-product")
	updated.Name = "Updated"
	s := &mockProductStore{
		updateFn: func(_ context.Context, slug, name, _ string) (*domain.Product, error) {
			if slug != "my-product" {
				t.Errorf("unexpected slug %q", slug)
			}
			return &updated, nil
		},
	}
	r := newRouter(s, editorIdentity("my-product"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "Updated", "description": "new desc"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProduct_SlugInBodyIsIgnored(t *testing.T) {
	updated := makeProduct("my-product")
	s := &mockProductStore{
		updateFn: func(_ context.Context, slug, _, _ string) (*domain.Product, error) {
			// Store receives the URL slug, not any slug from the body
			if slug != "my-product" {
				t.Errorf("store received slug %q, expected my-product", slug)
			}
			return &updated, nil
		},
	}
	r := newRouter(s, editorIdentity("my-product"))

	// Body includes a different "slug" field — it must be ignored
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "Name", "slug": "different-slug"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateProduct_ViewerForbidden(t *testing.T) {
	r := newRouter(&mockProductStore{}, viewerIdentity("my-product"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "X"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUpdateProduct_NoRoleReturns404(t *testing.T) {
	r := newRouter(&mockProductStore{}, noRoleIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/other-product",
		jsonBody(map[string]string{"name": "X"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// RequireRole middleware returns 404 for users with no role (anti-enumeration)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateProduct_StoreNotFoundReturns404(t *testing.T) {
	s := &mockProductStore{
		updateFn: func(_ context.Context, _, _, _ string) (*domain.Product, error) {
			return nil, store.ErrNotFound
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/ghost",
		jsonBody(map[string]string{"name": "X"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateProduct_MissingName_Returns422(t *testing.T) {
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"description": "no name provided"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

// --- ArchiveProduct tests ---

func TestArchiveProduct_EditorSoftDeletes(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, slug string) error {
			if slug != "my-product" {
				t.Errorf("unexpected slug %q", slug)
			}
			return nil
		},
	}
	r := newRouter(s, editorIdentity("my-product"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/my-product", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestArchiveProduct_ViewerForbidden(t *testing.T) {
	r := newRouter(&mockProductStore{}, viewerIdentity("my-product"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/my-product", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestArchiveProduct_NoRoleReturns404_AntiEnumeration(t *testing.T) {
	r := newRouter(&mockProductStore{}, noRoleIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/secret-product", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (anti-enumeration), got %d", w.Code)
	}
}

func TestArchiveProduct_AdminCanArchiveAnyProduct(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, slug string) error {
			return nil
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/any-product", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestArchiveProduct_StoreNotFoundReturns404(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, _ string) error {
			return store.ErrNotFound
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/ghost", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestArchiveProduct_InternalErrorReturns500(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, _ string) error {
			return errors.New("db is down")
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/any-product", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- toProductResponse ---

func TestToProductResponse_IncludesArchivedAt(t *testing.T) {
	archivedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	p := makeProduct("archived-slug")
	p.ArchivedAt = &archivedAt
	s := &mockProductStore{
		listFn: func(_ context.Context, _ store.ListOptions) ([]domain.Product, error) {
			return []domain.Product{p}, nil
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?include_archived=true", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 product, got %d", len(resp))
	}
	if resp[0]["archived_at"] == nil {
		t.Error("expected archived_at in response for archived product")
	}
}

// --- CreateProduct additional coverage ---

func TestCreateProduct_BadJSON_Returns400(t *testing.T) {
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

func TestCreateProduct_StoreInternalError_Returns500(t *testing.T) {
	s := &mockProductStore{
		createFn: func(_ context.Context, _ *domain.Product) error {
			return errors.New("db is down")
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "Prod", "slug": "prod"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- ListProducts additional coverage ---

func TestListProducts_NoIdentity_Returns401(t *testing.T) {
	r := newRouter(&mockProductStore{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when identity is absent, got %d", w.Code)
	}
}

func TestListProducts_StoreError_Returns500(t *testing.T) {
	s := &mockProductStore{
		listFn: func(_ context.Context, _ store.ListOptions) ([]domain.Product, error) {
			return nil, errors.New("db is down")
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on store error, got %d", w.Code)
	}
}

// --- UpdateProduct additional coverage ---

func TestUpdateProduct_BadJSON_Returns400(t *testing.T) {
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/my-product",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

func TestUpdateProduct_InvalidURLSlug_Returns400(t *testing.T) {
	// Slug "CAPS" fails domain.ValidateSlug; admin bypasses RequireRole so the
	// handler itself runs the slug validation.
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/CAPS",
		jsonBody(map[string]string{"name": "X"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL slug, got %d", w.Code)
	}
}

func TestUpdateProduct_StoreInternalError_Returns500(t *testing.T) {
	s := &mockProductStore{
		updateFn: func(_ context.Context, _, _, _ string) (*domain.Product, error) {
			return nil, errors.New("db is down")
		},
	}
	r := newRouter(s, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "X"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- ArchiveProduct additional coverage ---

func TestArchiveProduct_InvalidURLSlug_Returns400(t *testing.T) {
	r := newRouter(&mockProductStore{}, adminIdentity())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/CAPS", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL slug, got %d", w.Code)
	}
}
