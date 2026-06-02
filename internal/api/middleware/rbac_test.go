package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
)

// newRequireRoleEngine builds a test engine with a /products/:productSlug route
// protected by RequireRole(need). If identity is non-nil it is injected into the
// Gin context before the RBAC middleware runs (simulating upstream JWTAuth).
func newRequireRoleEngine(identity *domain.UserIdentity, need domain.Role) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/products/:productSlug")
	if identity != nil {
		id := identity
		group.Use(func(c *gin.Context) {
			c.Set(domain.IdentityContextKey, id)
			c.Next()
		})
	}
	group.Use(middleware.RequireRole(need))
	group.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name     string
		identity *domain.UserIdentity
		slug     string
		need     domain.Role
		wantCode int
	}{
		{
			name:     "devops admin bypasses editor requirement",
			identity: &domain.UserIdentity{IsDevOpsAdmin: true},
			slug:     "any-product",
			need:     domain.RoleEditor,
			wantCode: http.StatusOK,
		},
		{
			name:     "editor on target product satisfies editor requirement",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{"foo": domain.RoleEditor}},
			slug:     "foo",
			need:     domain.RoleEditor,
			wantCode: http.StatusOK,
		},
		{
			name:     "viewer on editor-required route returns 403",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{"foo": domain.RoleViewer}},
			slug:     "foo",
			need:     domain.RoleEditor,
			wantCode: http.StatusForbidden,
		},
		{
			name:     "user with role on different product returns 404",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{"other": domain.RoleEditor}},
			slug:     "foo",
			need:     domain.RoleEditor,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "empty product roles returns 404",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{}},
			slug:     "foo",
			need:     domain.RoleEditor,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "no identity in context returns 401",
			identity: nil,
			slug:     "foo",
			need:     domain.RoleEditor,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "editor satisfies viewer requirement",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{"foo": domain.RoleEditor}},
			slug:     "foo",
			need:     domain.RoleViewer,
			wantCode: http.StatusOK,
		},
		{
			name:     "viewer satisfies viewer requirement",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{"foo": domain.RoleViewer}},
			slug:     "foo",
			need:     domain.RoleViewer,
			wantCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRequireRoleEngine(tc.identity, tc.need)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/products/"+tc.slug, nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

// newRequireAdminEngine builds a test engine with a /admin route protected by
// RequireAdmin(). Identity injection follows the same pattern as above.
func newRequireAdminEngine(identity *domain.UserIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/admin")
	if identity != nil {
		id := identity
		group.Use(func(c *gin.Context) {
			c.Set(domain.IdentityContextKey, id)
			c.Next()
		})
	}
	group.Use(middleware.RequireAdmin())
	group.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name     string
		identity *domain.UserIdentity
		wantCode int
	}{
		{
			name:     "devops admin is allowed through",
			identity: &domain.UserIdentity{IsDevOpsAdmin: true},
			wantCode: http.StatusOK,
		},
		{
			name:     "editor without admin flag returns 403",
			identity: &domain.UserIdentity{ProductRoles: map[string]domain.Role{"foo": domain.RoleEditor}},
			wantCode: http.StatusForbidden,
		},
		{
			name:     "no identity in context returns 401",
			identity: nil,
			wantCode: http.StatusUnauthorized,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRequireAdminEngine(tc.identity)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}
