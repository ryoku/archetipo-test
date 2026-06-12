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

// --- mock environment store ---

type mockEnvironmentStore struct {
	createFn        func(ctx context.Context, e *domain.Environment) error
	listByProductFn func(ctx context.Context, productID string) ([]domain.Environment, error)
	getByIDFn       func(ctx context.Context, productID, environmentID string) (*domain.Environment, error)
	deleteFn        func(ctx context.Context, productID, environmentID string) error
}

func (m *mockEnvironmentStore) Create(ctx context.Context, e *domain.Environment) error {
	return m.createFn(ctx, e)
}
func (m *mockEnvironmentStore) ListByProduct(ctx context.Context, productID string) ([]domain.Environment, error) {
	return m.listByProductFn(ctx, productID)
}
func (m *mockEnvironmentStore) GetByID(ctx context.Context, productID, environmentID string) (*domain.Environment, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, productID, environmentID)
	}
	return nil, store.ErrEnvironmentNotFound
}
func (m *mockEnvironmentStore) Delete(ctx context.Context, productID, environmentID string) error {
	return m.deleteFn(ctx, productID, environmentID)
}

var _ store.EnvironmentStore = (*mockEnvironmentStore)(nil)

// --- router helper for environment tests ---

func newEnvironmentRouter(ps store.ProductStore, es store.EnvironmentStore, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewEnvironmentHandlers(ps, es)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	envs := api.Group("/products/:productSlug/environments")
	envs.POST("", middleware.RequireRole(domain.RoleEditor), h.CreateEnvironment)
	envs.GET("", middleware.RequireRole(domain.RoleViewer), h.ListEnvironments)
	envs.DELETE("/:environmentID", middleware.RequireRole(domain.RoleEditor), h.DeleteEnvironment)
	return r
}

// --- fixtures ---

func makeEnvironment(productID, id, envType string) domain.Environment {
	return domain.Environment{
		ID:          id,
		ProductID:   productID,
		Name:        "Env " + id,
		Type:        envType,
		OverlayPath: "overlays/" + id,
		Slug:        id,
		CreatedAt:   time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
	}
}

// --- CreateEnvironment tests ---

func TestCreateEnvironment_Valid_Returns201(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		createFn: func(_ context.Context, e *domain.Environment) error {
			e.ID = "new-env-id"
			e.CreatedAt = time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
			return nil
		},
	}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"slug":         "dev-env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusCreated)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["id"] == nil {
		t.Error("expected id in response")
	}
	if resp["slug"] != "dev-env" {
		t.Errorf("expected slug=dev-env in response, got %v", resp["slug"])
	}
}

func TestCreateEnvironment_NameMissing_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateEnvironment_InvalidType_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Staging Env",
			"type":         "staging",
			"overlay_path": "overlays/staging",
		}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateEnvironment_OverlayPathMissing_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"type":         "dev",
			"overlay_path": "",
		}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateEnvironment_AbsoluteOverlayPath_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"type":         "dev",
			"overlay_path": "/overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateEnvironment_NameConflict_Returns409(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		createFn: func(_ context.Context, _ *domain.Environment) error {
			return store.ErrEnvironmentNameConflict
		},
	}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Existing Env",
			"slug":         "existing-env",
			"type":         "integration",
			"overlay_path": "overlays/existing",
		}),
	)
	assertStatus(t, w, http.StatusConflict)
}

func TestCreateEnvironment_ProductNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugNotFound())
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, adminIdentity()),
		http.MethodPost,
		"/api/v1/products/ghost-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateEnvironment_ArchivedProduct_Returns404(t *testing.T) {
	archived := makeProduct("old-prod")
	archivedAt := time.Now()
	archived.ArchivedAt = &archivedAt
	ps := productStoreWithGetBySlug(func(_ context.Context, _ string) (*domain.Product, error) {
		return &archived, nil
	})
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("old-prod")),
		http.MethodPost,
		"/api/v1/products/old-prod/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateEnvironment_StoreError_Returns500(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		createFn: func(_ context.Context, _ *domain.Environment) error {
			return errors.New("db is down")
		},
	}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"slug":         "dev-env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestCreateEnvironment_ViewerForbidden_Returns403(t *testing.T) {
	ps := &mockProductStore{}
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, viewerIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusForbidden)
}

func TestCreateEnvironment_NoIdentity_Returns401(t *testing.T) {
	ps := &mockProductStore{}
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, nil),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestCreateEnvironment_EmptySlug_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"slug":         "",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateEnvironment_InvalidSlug_Returns422(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"slug":         "UPPERCASE_invalid!",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestCreateEnvironment_SlugConflict_Returns409(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		createFn: func(_ context.Context, _ *domain.Environment) error {
			return store.ErrEnvironmentSlugConflict
		},
	}
	w := doJSON(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodPost,
		"/api/v1/products/my-product/environments",
		jsonBody(map[string]string{
			"name":         "Dev Env",
			"slug":         "dev-env",
			"type":         "dev",
			"overlay_path": "overlays/dev",
		}),
	)
	assertStatus(t, w, http.StatusConflict)
}

// --- ListEnvironments tests ---

func TestListEnvironments_WithItems_Returns200(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	prod := makeProduct("my-product")
	es := &mockEnvironmentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Environment, error) {
			return []domain.Environment{
				makeEnvironment(prod.ID, "env-a", "dev"),
				makeEnvironment(prod.ID, "env-b", "production"),
			}, nil
		},
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/environments",
	)
	assertStatus(t, w, http.StatusOK)

	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 environments, got %d", len(resp))
	}
}

func TestListEnvironments_EmptyList_Returns200WithArray(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Environment, error) {
			return []domain.Environment{}, nil
		},
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/environments",
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

func TestListEnvironments_ProductNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugNotFound())
	es := &mockEnvironmentStore{}
	w := doPlain(
		newEnvironmentRouter(ps, es, adminIdentity()),
		http.MethodGet,
		"/api/v1/products/ghost-product/environments",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestListEnvironments_ArchivedProduct_Returns404(t *testing.T) {
	archived := makeProduct("old-prod")
	archivedAt := time.Now()
	archived.ArchivedAt = &archivedAt
	ps := productStoreWithGetBySlug(func(_ context.Context, _ string) (*domain.Product, error) {
		return &archived, nil
	})
	w := doPlain(
		newEnvironmentRouter(ps, &mockEnvironmentStore{}, viewerIdentity("old-prod")),
		http.MethodGet,
		"/api/v1/products/old-prod/environments",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestListEnvironments_StoreError_Returns500(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		listByProductFn: func(_ context.Context, _ string) ([]domain.Environment, error) {
			return nil, errors.New("db is down")
		},
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/environments",
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

// --- DeleteEnvironment tests ---

func TestDeleteEnvironment_Success_Returns204(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/environments/env-id-123",
	)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteEnvironment_EnvironmentNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return store.ErrEnvironmentNotFound },
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/environments/ghost-env-id",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteEnvironment_HasDeployments_Returns409(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return store.ErrEnvironmentHasDeployments },
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/environments/busy-env-id",
	)
	assertStatus(t, w, http.StatusConflict)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "environment has active deployment records" {
		t.Errorf("unexpected error message: %v", resp["error"])
	}
}

func TestDeleteEnvironment_ProductNotFound_Returns404(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugNotFound())
	es := &mockEnvironmentStore{}
	w := doPlain(
		newEnvironmentRouter(ps, es, adminIdentity()),
		http.MethodDelete,
		"/api/v1/products/ghost-product/environments/env-id-123",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteEnvironment_ViewerForbidden_Returns403(t *testing.T) {
	ps := &mockProductStore{}
	es := &mockEnvironmentStore{}
	w := doPlain(
		newEnvironmentRouter(ps, es, viewerIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/environments/env-id-123",
	)
	assertStatus(t, w, http.StatusForbidden)
}

func TestDeleteEnvironment_ArchivedProduct_Returns404(t *testing.T) {
	archived := makeProduct("old-prod")
	archivedAt := time.Now()
	archived.ArchivedAt = &archivedAt
	ps := productStoreWithGetBySlug(func(_ context.Context, _ string) (*domain.Product, error) {
		return &archived, nil
	})
	es := &mockEnvironmentStore{}
	w := doPlain(
		newEnvironmentRouter(ps, es, editorIdentity("old-prod")),
		http.MethodDelete,
		"/api/v1/products/old-prod/environments/env-id-123",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteEnvironment_StoreError_Returns500(t *testing.T) {
	ps := productStoreWithGetBySlug(productGetBySlugOK("my-product"))
	es := &mockEnvironmentStore{
		deleteFn: func(_ context.Context, _, _ string) error { return errors.New("db is down") },
	}
	w := doPlain(
		newEnvironmentRouter(ps, es, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/environments/env-id-123",
	)
	assertStatus(t, w, http.StatusInternalServerError)
}
