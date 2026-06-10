package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterDeploymentRoutes returns a route registration function for the deploy endpoint.
func RegisterDeploymentRoutes(
	ps store.ProductStore,
	es store.EnvironmentStore,
	cs store.ComponentStore,
	ls store.DeploymentLockStore,
	applier handlers.GitOpsApplier,
) func(*gin.RouterGroup) {
	h := handlers.NewDeploymentHandlers(ps, es, cs, ls, applier)
	return func(api *gin.RouterGroup) {
		api.POST(
			"/products/:productSlug/environments/:environmentID/deployments",
			middleware.RequireRole(domain.RoleEditor),
			h.Deploy,
		)
	}
}
