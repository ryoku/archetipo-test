package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterEnvironmentRoutes returns a route registration function for /products/:productSlug/environments.
func RegisterEnvironmentRoutes(ps store.ProductStore, es store.EnvironmentStore) func(*gin.RouterGroup) {
	h := handlers.NewEnvironmentHandlers(ps, es)
	return func(api *gin.RouterGroup) {
		envs := api.Group("/products/:productSlug/environments")
		envs.POST("", middleware.RequireRole(domain.RoleEditor), h.CreateEnvironment)
		envs.GET("", middleware.RequireRole(domain.RoleViewer), h.ListEnvironments)
		envs.DELETE("/:environmentID", middleware.RequireRole(domain.RoleEditor), h.DeleteEnvironment)
	}
}
