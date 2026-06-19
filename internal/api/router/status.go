package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterStatusRoutes registers GET /products/:productSlug/status.
func RegisterStatusRoutes(ps store.ProductStore, es store.EnvironmentStore, r gitops.StatusReader) func(*gin.RouterGroup) {
	h := handlers.NewStatusHandlers(ps, es, r)
	return func(api *gin.RouterGroup) {
		api.GET("/products/:productSlug/status",
			middleware.RequireRole(domain.RoleViewer),
			h.GetStatus,
		)
	}
}
