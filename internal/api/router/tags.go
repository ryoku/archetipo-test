package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterTagRoutes registers GET /products/:productSlug/environments/:environmentID/workloads/:workload/tags.
func RegisterTagRoutes(ps store.ProductStore, es store.EnvironmentStore, r gitops.WorkloadReader, l gcr.Lister) func(*gin.RouterGroup) {
	h := handlers.NewTagHandlers(ps, es, r, l)
	return func(api *gin.RouterGroup) {
		api.GET("/products/:productSlug/environments/:environmentID/workloads/:workload/tags",
			middleware.RequireRole(domain.RoleViewer),
			h.ListTags,
		)
	}
}
