package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterProductRoutes returns a route registration function for /products endpoints.
// Caller passes this to router.New(..., RegisterProductRoutes(s)).
func RegisterProductRoutes(s store.ProductStore) func(*gin.RouterGroup) {
	h := handlers.NewProductHandlers(s)
	return func(api *gin.RouterGroup) {
		products := api.Group("/products")
		{
			// POST: DevOps Admin only — creates a product
			products.POST("", middleware.RequireAdmin(), h.CreateProduct)
			// GET: any authenticated user — handler filters by identity
			products.GET("", h.ListProducts)
			// PUT/DELETE: per-product role check (editor minimum); anti-enumeration 404 for no-role users
			products.PUT("/:productSlug", middleware.RequireRole(domain.RoleEditor), h.UpdateProduct)
			products.DELETE("/:productSlug", middleware.RequireRole(domain.RoleEditor), h.ArchiveProduct)
		}
	}
}
