package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterTagConventionRoutes returns a route registration function for
// /products/:productSlug/tag-convention endpoints.
func RegisterTagConventionRoutes(s store.ProductStore, defaultRegex string) func(*gin.RouterGroup) {
	h := handlers.NewTagConventionHandlers(s, defaultRegex)
	return func(api *gin.RouterGroup) {
		tc := api.Group("/products/:productSlug/tag-convention")
		// GET: Viewer and above — consistent with components and environments router pattern
		tc.GET("", middleware.RequireRole(domain.RoleViewer), h.GetTagConvention)
		// PUT: Editor (or DevOps Admin) only
		tc.PUT("", middleware.RequireRole(domain.RoleEditor), h.PutTagConvention)
	}
}
