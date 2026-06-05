package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterComponentRoutes returns a route registration function for /products/:productSlug/components.
func RegisterComponentRoutes(ps store.ProductStore, cs store.ComponentStore) func(*gin.RouterGroup) {
	h := handlers.NewComponentHandlers(ps, cs)
	return func(api *gin.RouterGroup) {
		comps := api.Group("/products/:productSlug/components")
		comps.POST("", middleware.RequireRole(domain.RoleEditor), h.CreateComponent)
		comps.GET("", middleware.RequireRole(domain.RoleViewer), h.ListComponents)
		comps.DELETE("/:componentSlug", middleware.RequireRole(domain.RoleEditor), h.DeleteComponent)
	}
}
