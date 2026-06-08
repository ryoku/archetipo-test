package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterTagRoutes returns a route registration function for
// GET /products/:productSlug/components/:componentSlug/tags.
func RegisterTagRoutes(ps store.ProductStore, cs store.ComponentStore, l gcr.Lister) func(*gin.RouterGroup) {
	h := handlers.NewTagHandlers(ps, cs, l)
	return func(api *gin.RouterGroup) {
		api.GET("/products/:productSlug/components/:componentSlug/tags",
			middleware.RequireRole(domain.RoleViewer),
			h.ListTags,
		)
	}
}
