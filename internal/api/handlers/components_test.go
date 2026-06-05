package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// --- mock component store ---

type mockComponentStore struct {
	createFn        func(ctx context.Context, c *domain.Component) error
	listByProductFn func(ctx context.Context, productID string) ([]domain.Component, error)
	deleteFn        func(ctx context.Context, productID, slug string) error
}

func (m *mockComponentStore) Create(ctx context.Context, c *domain.Component) error {
	return m.createFn(ctx, c)
}
func (m *mockComponentStore) ListByProduct(ctx context.Context, productID string) ([]domain.Component, error) {
	return m.listByProductFn(ctx, productID)
}
func (m *mockComponentStore) Delete(ctx context.Context, productID, slug string) error {
	return m.deleteFn(ctx, productID, slug)
}

var _ store.ComponentStore = (*mockComponentStore)(nil)

// --- router helper for component tests ---

func newComponentRouter(ps store.ProductStore, cs store.ComponentStore, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewComponentHandlers(ps, cs)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	comps := api.Group("/products/:productSlug/components")
	comps.POST("", middleware.RequireRole(domain.RoleEditor), h.CreateComponent)
	comps.GET("", middleware.RequireRole(domain.RoleViewer), h.ListComponents)
	comps.DELETE("/:componentSlug", middleware.RequireRole(domain.RoleEditor), h.DeleteComponent)
	return r
}

// --- fixtures ---

func makeComponent(productID, slug string) domain.Component {
	return domain.Component{
		ID:           "comp-id-" + slug,
		ProductID:    productID,
		Name:         "Component " + slug,
		Slug:         slug,
		GCRImagePath: "gcr.io/project/" + slug,
		CreatedAt:    time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
	}
}

func productGetBySlugOK(slug string) func(ctx context.Context, s string) (*domain.Product, error) {
	p := makeProduct(slug)
	return func(_ context.Context, _ string) (*domain.Product, error) {
		return &p, nil
	}
}

func productGetBySlugNotFound() func(ctx context.Context, s string) (*domain.Product, error) {
	return func(_ context.Context, _ string) (*domain.Product, error) {
		return nil, store.ErrNotFound
	}
}

// productStoreWithGetBySlug returns a mockProductStore with only GetBySlug wired.
func productStoreWithGetBySlug(fn func(ctx context.Context, slug string) (*domain.Product, error)) *mockProductStore {
	return &mockProductStore{getBySlugFn: fn}
}

// --- CreateComponent tests ---

func TestCreateComponent_Valid_Returns201(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		createFn: func(_ context.Context, c *domain.Component) error {
			c.ID = "new-comp-id"
			c.CreatedAt = time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
			return nil
		},
	}
	w := doJSON(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "My Comp", "slug": "my-comp", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusCreated)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestCreateComponent_NameEmpty_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{}
	w := doJSON(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "", "slug": "valid-slug", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateComponent_InvalidSlug_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{}
	w := doJSON(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "Comp", "slug": "UPPER", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateComponent_GCRImagePathEmpty_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{}
	w := doJSON(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "Comp", "slug": "valid-slug", "gcr_image_path": ""}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateComponent_ProductNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugNotFound())
	cs := &mockComponentStore{}
	w := doJSON(
		newComponentRouter(ps, cs, adminIdentity()),
		http.MethodPost,
		"/api/v1/products/ghost-product/components",
		jsonBody(map[string]string{"name": "Comp", "slug": "valid-slug", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateComponent_SlugConflict_Returns409(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		createFn: func(_ context.Context, _ *domain.Component) error {
			return store.ErrComponentSlugConflict
		},
	}
	w := doJSON(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "Comp", "slug": "existing", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusConflict)
}

func TestCreateComponent_StoreError_Returns500(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		createFn: func(_ context.Context, _ *domain.Component) error {
			return errors.New("db is down")
		},
	}
	w := doJSON(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "Comp", "slug": "new-comp", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestCreateComponent_NoIdentity_Returns401(t *testing.T) {
	ps := &mockProductStore{}
	cs := &mockComponentStore{}
	w := doJSON(
		newComponentRouter(ps, cs, nil),
		http.MethodPost,
		"/api/v1/products/my-product/components",
		jsonBody(map[string]string{"name": "Comp", "slug": "comp", "gcr_image_path": "gcr.io/p/img"}),
	)
	assertStatus(t, w, http.StatusUnauthorized)
}

// --- ListComponents tests ---

func TestListComponents_EmptyList_Returns200WithArray(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Component, error) {
			return []domain.Component{}, nil
		},
	}
	w := doPlain(
		newComponentRouter(ps, cs, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/components",
	)
	assertStatus(t, w, http.StatusOK)

	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestListComponents_WithItems_Returns200WithArray(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	prod := makeProduct("my-product")
	cs := &mockComponentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Component, error) {
			return []domain.Component{
				makeComponent(prod.ID, "comp-a"),
				makeComponent(prod.ID, "comp-b"),
			}, nil
		},
	}
	w := doPlain(
		newComponentRouter(ps, cs, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/components",
	)
	assertStatus(t, w, http.StatusOK)

	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 components, got %d", len(resp))
	}
}

func TestListComponents_ProductNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugNotFound())
	cs := &mockComponentStore{}
	w := doPlain(
		newComponentRouter(ps, cs, adminIdentity()),
		http.MethodGet,
		"/api/v1/products/ghost-product/components",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestListComponents_StoreError_Returns500(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Component, error) {
			return nil, errors.New("db is down")
		},
	}
	w := doPlain(
		newComponentRouter(ps, cs, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/components",
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

// --- DeleteComponent tests ---

func TestDeleteComponent_Success_Returns204(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	w := doPlain(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/components/my-comp",
	)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteComponent_ComponentNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return store.ErrComponentNotFound },
	}
	w := doPlain(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/components/ghost-comp",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteComponent_HasDeployments_Returns409(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return store.ErrComponentHasDeployments },
	}
	w := doPlain(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/components/busy-comp",
	)
	assertStatus(t, w, http.StatusConflict)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "component has active deployment records" {
		t.Errorf("unexpected error message: %v", resp["error"])
	}
}

func TestDeleteComponent_ProductNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugNotFound())
	cs := &mockComponentStore{}
	w := doPlain(
		newComponentRouter(ps, cs, adminIdentity()),
		http.MethodDelete,
		"/api/v1/products/ghost-product/components/any-comp",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteComponent_ViewerForbidden_Returns403(t *testing.T) {
	ps := &mockProductStore{}
	cs := &mockComponentStore{}
	w := doPlain(
		newComponentRouter(ps, cs, viewerIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/components/some-comp",
	)
	assertStatus(t, w, http.StatusForbidden)
}

func TestDeleteComponent_StoreError_Returns500(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	cs := &mockComponentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return errors.New("db is down") },
	}
	w := doPlain(
		newComponentRouter(ps, cs, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/components/some-comp",
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

