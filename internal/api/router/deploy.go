package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

func RegisterDeploymentRoutes(
	ps store.ProductStore,
	es store.EnvironmentStore,
	ls store.DeploymentLockStore,
	ds store.DeploymentStore,
	applier handlers.GitOpsApplier,
	defaultTagConvention string,
) func(*gin.RouterGroup) {
	h := handlers.NewDeploymentHandlers(ps, es, ls, ds, applier, defaultTagConvention)
	return func(api *gin.RouterGroup) {
		api.POST(
			"/products/:productSlug/environments/:environmentID/deployments",
			middleware.RequireRole(domain.RoleEditor),
			h.Deploy,
		)
		api.GET("/deployments/:deploymentID", h.GetDeployment)
	}
}
