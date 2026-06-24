package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterAdminRoutes returns a route registration function for /admin endpoints.
func RegisterAdminRoutes(ps store.ProductStore) func(*gin.RouterGroup) {
	h := handlers.NewAdminHandlers(ps)
	return func(api *gin.RouterGroup) {
		admin := api.Group("/admin", middleware.RequireAdmin())
		{
			admin.GET("/products", h.GetAdminProducts)
		}
	}
}
