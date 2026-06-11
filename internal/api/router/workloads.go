package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterWorkloadRoutes registers GET /products/:productSlug/environments/:environmentID/workloads.
func RegisterWorkloadRoutes(ps store.ProductStore, es store.EnvironmentStore, r gitops.WorkloadReader) func(*gin.RouterGroup) {
	h := handlers.NewWorkloadHandlers(ps, es, r)
	return func(api *gin.RouterGroup) {
		api.GET("/products/:productSlug/environments/:environmentID/workloads",
			middleware.RequireRole(domain.RoleViewer),
			h.ListWorkloads,
		)
	}
}
