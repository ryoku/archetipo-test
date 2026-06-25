package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/store"
)

// RegisterStatsRoutes returns a route registration function for GET /stats.
func RegisterStatsRoutes(s store.StatsStore) func(*gin.RouterGroup) {
	h := handlers.NewStatsHandlers(s)
	return func(api *gin.RouterGroup) {
		api.GET("/stats", h.GetStats)
	}
}
