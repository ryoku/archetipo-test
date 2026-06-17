package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterStatusRoutes returns a route registration function for the deployment status endpoint.
func RegisterStatusRoutes(ps store.ProductStore, es store.EnvironmentStore, r handlers.StatusReader) func(*gin.RouterGroup) {
	h := handlers.NewStatusHandlers(ps, es, r)
	return func(api *gin.RouterGroup) {
		api.GET(
			"/products/:productSlug/status",
			middleware.RequireRole(domain.RoleViewer),
			h.GetStatus,
		)
	}
}
