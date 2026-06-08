package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

const defaultTestRegex = `^v\d+\.\d+\.\d+$`

// --- router helper for tag-convention tests ---

func newTagConventionRouter(ps store.ProductStore, identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewTagConventionHandlers(ps, defaultTestRegex)

	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}

	api := r.Group("/api/v1", injectIdentity)
	tc := api.Group("/products/:productSlug/tag-convention")
	tc.GET("", middleware.RequireRole(domain.RoleViewer), h.GetTagConvention)
	tc.PUT("", middleware.RequireRole(domain.RoleEditor), h.PutTagConvention)
	tc.DELETE("", middleware.RequireRole(domain.RoleEditor), h.DeleteTagConvention)
	return r
}

// tagConventionStore returns a mockProductStore with GetTagConvention and SetTagConvention wired.
func tagConventionStore(
	getTagConventionFn func(ctx context.Context, slug string) (*string, error),
	setTagConventionFn func(ctx context.Context, slug, regex string) error,
) *mockProductStore {
	return &mockProductStore{
		getTagConventionFn: getTagConventionFn,
		setTagConventionFn: setTagConventionFn,
	}
}

// clearTagConventionStore returns a mockProductStore with ClearTagConvention wired.
func clearTagConventionStore(
	clearTagConventionFn func(ctx context.Context, slug string) error,
) *mockProductStore {
	return &mockProductStore{
		clearTagConventionFn: clearTagConventionFn,
	}
}

// --- GetTagConvention tests ---

func TestGetTagConvention_Unauthenticated_Returns401(t *testing.T) {
	ps := tagConventionStore(nil, nil)
	// Pass nil identity so the injectIdentity middleware skips setting it in the context.
	w := doPlain(
		newTagConventionRouter(ps, nil),
		http.MethodGet,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestPutTagConvention_ViewerRole_Returns403(t *testing.T) {
	ps := tagConventionStore(nil, nil)
	w := doJSON(
		newTagConventionRouter(ps, viewerIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]string{"regex": `^v\d+$`}),
	)
	assertStatus(t, w, http.StatusForbidden)
}

func TestGetTagConvention_ProductOverrideExists_Returns200WithSourceProduct(t *testing.T) {
	override := `^release-\d+$`
	ps := tagConventionStore(
		func(_ context.Context, slug string) (*string, error) {
			if slug != "my-product" {
				t.Errorf("unexpected slug %q", slug)
			}
			return &override, nil
		},
		nil,
	)
	w := doPlain(
		newTagConventionRouter(ps, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusOK)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["regex"] != override {
		t.Errorf("expected regex %q, got %v", override, resp["regex"])
	}
	if resp["source"] != "product" {
		t.Errorf("expected source %q, got %v", "product", resp["source"])
	}
}

func TestGetTagConvention_NoOverride_Returns200WithDefaultRegexAndSourceDefault(t *testing.T) {
	ps := tagConventionStore(
		func(_ context.Context, _ string) (*string, error) {
			return nil, nil
		},
		nil,
	)
	w := doPlain(
		newTagConventionRouter(ps, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusOK)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["regex"] != defaultTestRegex {
		t.Errorf("expected default regex %q, got %v", defaultTestRegex, resp["regex"])
	}
	if resp["source"] != "default" {
		t.Errorf("expected source %q, got %v", "default", resp["source"])
	}
}

func TestGetTagConvention_UnknownProduct_Returns404(t *testing.T) {
	ps := tagConventionStore(
		func(_ context.Context, _ string) (*string, error) {
			return nil, store.ErrNotFound
		},
		nil,
	)
	w := doPlain(
		newTagConventionRouter(ps, adminIdentity()),
		http.MethodGet,
		"/api/v1/products/ghost-product/tag-convention",
	)
	assertStatus(t, w, http.StatusNotFound)
}

// --- PutTagConvention tests ---

func TestPutTagConvention_ValidRegex_Returns200WithSourceProduct(t *testing.T) {
	stored := ""
	newRegex := `^hotfix-\d+\.\d+$`
	ps := tagConventionStore(
		nil,
		func(_ context.Context, slug, regex string) error {
			if slug != "my-product" {
				t.Errorf("unexpected slug %q", slug)
			}
			stored = regex
			return nil
		},
	)
	w := doJSON(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]string{"regex": newRegex}),
	)
	assertStatus(t, w, http.StatusOK)

	if stored != newRegex {
		t.Errorf("store received regex %q, expected %q", stored, newRegex)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["regex"] != newRegex {
		t.Errorf("expected regex %q in response, got %v", newRegex, resp["regex"])
	}
	if resp["source"] != "product" {
		t.Errorf("expected source %q, got %v", "product", resp["source"])
	}
}

func TestPutTagConvention_InvalidGoRegex_Returns400(t *testing.T) {
	ps := tagConventionStore(nil, nil)
	w := doJSON(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]string{"regex": "[invalid("}),
	)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestPutTagConvention_EmptyRegexField_Returns422(t *testing.T) {
	ps := tagConventionStore(nil, nil)
	w := doJSON(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]string{"regex": ""}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestPutTagConvention_UnknownProduct_Returns404(t *testing.T) {
	ps := tagConventionStore(
		nil,
		func(_ context.Context, _, _ string) error {
			return store.ErrNotFound
		},
	)
	w := doJSON(
		newTagConventionRouter(ps, adminIdentity()),
		http.MethodPut,
		"/api/v1/products/ghost-product/tag-convention",
		jsonBody(map[string]string{"regex": `^v\d+$`}),
	)
	assertStatus(t, w, http.StatusNotFound)
}

// --- DeleteTagConvention tests ---

func TestDeleteTagConvention_Success_Returns204(t *testing.T) {
	called := false
	ps := clearTagConventionStore(
		func(_ context.Context, slug string) error {
			if slug != "my-product" {
				t.Errorf("unexpected slug %q", slug)
			}
			called = true
			return nil
		},
	)
	w := doPlain(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusNoContent)
	if !called {
		t.Error("expected ClearTagConvention to be called")
	}
}

func TestDeleteTagConvention_UnknownProduct_Returns404(t *testing.T) {
	ps := clearTagConventionStore(
		func(_ context.Context, _ string) error {
			return store.ErrNotFound
		},
	)
	w := doPlain(
		newTagConventionRouter(ps, adminIdentity()),
		http.MethodDelete,
		"/api/v1/products/ghost-product/tag-convention",
	)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteTagConvention_ViewerRole_Returns403(t *testing.T) {
	ps := clearTagConventionStore(nil)
	w := doPlain(
		newTagConventionRouter(ps, viewerIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusForbidden)
}

func TestPutTagConvention_MissingRegexKey_Returns422(t *testing.T) {
	ps := tagConventionStore(nil, nil)
	w := doJSON(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]any{}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestPutTagConvention_RegexTooLong_Returns422(t *testing.T) {
	ps := tagConventionStore(nil, nil)
	w := doJSON(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]string{"regex": strings.Repeat("a", 501)}),
	)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

func TestGetTagConvention_StoreError_Returns500(t *testing.T) {
	ps := tagConventionStore(
		func(_ context.Context, _ string) (*string, error) {
			return nil, fmt.Errorf("db timeout")
		},
		nil,
	)
	w := doPlain(
		newTagConventionRouter(ps, viewerIdentity("my-product")),
		http.MethodGet,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestPutTagConvention_StoreError_Returns500(t *testing.T) {
	ps := tagConventionStore(
		nil,
		func(_ context.Context, _, _ string) error {
			return fmt.Errorf("db timeout")
		},
	)
	w := doJSON(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodPut,
		"/api/v1/products/my-product/tag-convention",
		jsonBody(map[string]string{"regex": `^v\d+$`}),
	)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestDeleteTagConvention_StoreError_Returns500(t *testing.T) {
	ps := clearTagConventionStore(
		func(_ context.Context, _ string) error {
			return fmt.Errorf("db timeout")
		},
	)
	w := doPlain(
		newTagConventionRouter(ps, editorIdentity("my-product")),
		http.MethodDelete,
		"/api/v1/products/my-product/tag-convention",
	)
	assertStatus(t, w, http.StatusInternalServerError)
}
