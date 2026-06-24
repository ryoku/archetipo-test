package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

func RegisterHistoryRoutes(ps store.ProductStore, ds store.DeploymentStore) func(*gin.RouterGroup) {
	h := handlers.NewHistoryHandlers(ps, ds)
	return func(api *gin.RouterGroup) {
		api.GET(
			"/products/:productSlug/deployments",
			middleware.RequireRole(domain.RoleViewer),
			h.ListByProduct,
		)
		api.GET(
			"/admin/deployments",
			middleware.RequireAdmin(),
			h.ListAll,
		)
	}
}
