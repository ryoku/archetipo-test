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
	createFn             func(ctx context.Context, p *domain.Product) error
	listFn               func(ctx context.Context, opts store.ListOptions) ([]domain.Product, error)
	getBySlugFn          func(ctx context.Context, slug string) (*domain.Product, error)
	getByIDFn            func(ctx context.Context, id string) (*domain.Product, error)
	updateFn             func(ctx context.Context, slug, name, description string) (*domain.Product, error)
	archiveFn            func(ctx context.Context, slug string) error
	getTagConventionFn   func(ctx context.Context, slug string) (*string, error)
	setTagConventionFn   func(ctx context.Context, slug, regex string) error
	clearTagConventionFn func(ctx context.Context, slug string) error
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
func (m *mockProductStore) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, store.ErrNotFound
}
func (m *mockProductStore) Update(ctx context.Context, slug, name, description string) (*domain.Product, error) {
	return m.updateFn(ctx, slug, name, description)
}
func (m *mockProductStore) Archive(ctx context.Context, slug string) error {
	return m.archiveFn(ctx, slug)
}
func (m *mockProductStore) GetTagConvention(ctx context.Context, slug string) (*string, error) {
	return m.getTagConventionFn(ctx, slug)
}
func (m *mockProductStore) SetTagConvention(ctx context.Context, slug, regex string) error {
	return m.setTagConventionFn(ctx, slug, regex)
}
func (m *mockProductStore) ClearTagConvention(ctx context.Context, slug string) error {
	return m.clearTagConventionFn(ctx, slug)
}

var _ store.ProductStore = (*mockProductStore)(nil)

// --- test helpers ---

func newRouter(s store.ProductStore, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewProductHandlers(s)

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

// doJSON sends method to path with a JSON body and returns the recorder.
func doJSON(r *gin.Engine, method, path string, body *bytes.Buffer) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// doPlain sends method to path with no body and returns the recorder.
func doPlain(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// assertStatus fails the test if the recorded status differs from want.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, w.Code, w.Body.String())
	}
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
	w := doJSON(newRouter(s, adminIdentity()), http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "My Product", "slug": "my-product", "description": "desc"}))
	assertStatus(t, w, http.StatusCreated)
}

func TestCreateProduct_InvalidSlug_Returns422(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, adminIdentity()), http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "Bad Slug", "slug": "Bad Slug!"}))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateProduct_SlugConflict_Returns409(t *testing.T) {
	s := &mockProductStore{
		createFn: func(_ context.Context, _ *domain.Product) error {
			return store.ErrSlugConflict
		},
	}
	w := doJSON(newRouter(s, adminIdentity()), http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "Dupe", "slug": "existing-slug"}))
	assertStatus(t, w, http.StatusConflict)
}

func TestCreateProduct_NonAdminForbidden(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, editorIdentity("some-product")), http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "X", "slug": "x"}))
	assertStatus(t, w, http.StatusForbidden)
}

func TestCreateProduct_MissingName_Returns422(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, adminIdentity()), http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"slug": "valid-slug"}))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateProduct_BadJSON_Returns400(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, adminIdentity()), http.MethodPost, "/api/v1/products",
		bytes.NewBufferString("not-json"))
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateProduct_StoreInternalError_Returns500(t *testing.T) {
	s := &mockProductStore{
		createFn: func(_ context.Context, _ *domain.Product) error {
			return errors.New("db is down")
		},
	}
	w := doJSON(newRouter(s, adminIdentity()), http.MethodPost, "/api/v1/products",
		jsonBody(map[string]string{"name": "Prod", "slug": "prod"}))
	assertStatus(t, w, http.StatusInternalServerError)
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
	w := doPlain(newRouter(s, adminIdentity()), http.MethodGet, "/api/v1/products?include_archived=true")
	assertStatus(t, w, http.StatusOK)

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
	w := doPlain(newRouter(s, adminIdentity()), http.MethodGet, "/api/v1/products")
	assertStatus(t, w, http.StatusOK)

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
	w := doPlain(newRouter(s, editorIdentity("my-product")), http.MethodGet, "/api/v1/products")
	assertStatus(t, w, http.StatusOK)
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
	w := doPlain(newRouter(s, adminIdentity()), http.MethodGet, "/api/v1/products?include_archived=true")
	assertStatus(t, w, http.StatusOK)
}

func TestListProducts_NoRoleUserReceivesEmptyList(t *testing.T) {
	s := &mockProductStore{
		listFn: func(_ context.Context, _ store.ListOptions) ([]domain.Product, error) {
			return []domain.Product{}, nil
		},
	}
	w := doPlain(newRouter(s, noRoleIdentity()), http.MethodGet, "/api/v1/products")
	assertStatus(t, w, http.StatusOK)

	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestListProducts_NoIdentity_Returns401(t *testing.T) {
	w := doPlain(newRouter(&mockProductStore{}, nil), http.MethodGet, "/api/v1/products")
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestListProducts_StoreError_Returns500(t *testing.T) {
	s := &mockProductStore{
		listFn: func(_ context.Context, _ store.ListOptions) ([]domain.Product, error) {
			return nil, errors.New("db is down")
		},
	}
	w := doPlain(newRouter(s, adminIdentity()), http.MethodGet, "/api/v1/products")
	assertStatus(t, w, http.StatusInternalServerError)
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
	w := doJSON(newRouter(s, editorIdentity("my-product")), http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "Updated", "description": "new desc"}))
	assertStatus(t, w, http.StatusOK)
}

func TestUpdateProduct_SlugInBodyIsIgnored(t *testing.T) {
	updated := makeProduct("my-product")
	s := &mockProductStore{
		updateFn: func(_ context.Context, slug, _, _ string) (*domain.Product, error) {
			if slug != "my-product" {
				t.Errorf("store received slug %q, expected my-product", slug)
			}
			return &updated, nil
		},
	}
	w := doJSON(newRouter(s, editorIdentity("my-product")), http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "Name", "slug": "different-slug"}))
	assertStatus(t, w, http.StatusOK)
}

func TestUpdateProduct_ViewerForbidden(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, viewerIdentity("my-product")), http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "X"}))
	assertStatus(t, w, http.StatusForbidden)
}

func TestUpdateProduct_NoRoleReturns404(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, noRoleIdentity()), http.MethodPut, "/api/v1/products/other-product",
		jsonBody(map[string]string{"name": "X"}))
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdateProduct_StoreNotFoundReturns404(t *testing.T) {
	s := &mockProductStore{
		updateFn: func(_ context.Context, _, _, _ string) (*domain.Product, error) {
			return nil, store.ErrNotFound
		},
	}
	w := doJSON(newRouter(s, adminIdentity()), http.MethodPut, "/api/v1/products/ghost",
		jsonBody(map[string]string{"name": "X"}))
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdateProduct_MissingName_Returns422(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, adminIdentity()), http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"description": "no name provided"}))
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestUpdateProduct_BadJSON_Returns400(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, adminIdentity()), http.MethodPut, "/api/v1/products/my-product",
		bytes.NewBufferString("not-json"))
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateProduct_InvalidURLSlug_Returns400(t *testing.T) {
	w := doJSON(newRouter(&mockProductStore{}, adminIdentity()), http.MethodPut, "/api/v1/products/CAPS",
		jsonBody(map[string]string{"name": "X"}))
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateProduct_StoreInternalError_Returns500(t *testing.T) {
	s := &mockProductStore{
		updateFn: func(_ context.Context, _, _, _ string) (*domain.Product, error) {
			return nil, errors.New("db is down")
		},
	}
	w := doJSON(newRouter(s, adminIdentity()), http.MethodPut, "/api/v1/products/my-product",
		jsonBody(map[string]string{"name": "X"}))
	assertStatus(t, w, http.StatusInternalServerError)
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
	w := doPlain(newRouter(s, editorIdentity("my-product")), http.MethodDelete, "/api/v1/products/my-product")
	assertStatus(t, w, http.StatusNoContent)
}

func TestArchiveProduct_ViewerForbidden(t *testing.T) {
	w := doPlain(newRouter(&mockProductStore{}, viewerIdentity("my-product")), http.MethodDelete, "/api/v1/products/my-product")
	assertStatus(t, w, http.StatusForbidden)
}

func TestArchiveProduct_NoRoleReturns404_AntiEnumeration(t *testing.T) {
	w := doPlain(newRouter(&mockProductStore{}, noRoleIdentity()), http.MethodDelete, "/api/v1/products/secret-product")
	assertStatus(t, w, http.StatusNotFound)
}

func TestArchiveProduct_AdminCanArchiveAnyProduct(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, _ string) error { return nil },
	}
	w := doPlain(newRouter(s, adminIdentity()), http.MethodDelete, "/api/v1/products/any-product")
	assertStatus(t, w, http.StatusNoContent)
}

func TestArchiveProduct_StoreNotFoundReturns404(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, _ string) error { return store.ErrNotFound },
	}
	w := doPlain(newRouter(s, adminIdentity()), http.MethodDelete, "/api/v1/products/ghost")
	assertStatus(t, w, http.StatusNotFound)
}

func TestArchiveProduct_InternalErrorReturns500(t *testing.T) {
	s := &mockProductStore{
		archiveFn: func(_ context.Context, _ string) error { return errors.New("db is down") },
	}
	w := doPlain(newRouter(s, adminIdentity()), http.MethodDelete, "/api/v1/products/any-product")
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestArchiveProduct_InvalidURLSlug_Returns400(t *testing.T) {
	w := doPlain(newRouter(&mockProductStore{}, adminIdentity()), http.MethodDelete, "/api/v1/products/CAPS")
	assertStatus(t, w, http.StatusBadRequest)
}
