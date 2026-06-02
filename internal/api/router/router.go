package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/auth"
)

// New returns a configured Gin engine with:
//   - GET /healthz (unauthenticated)
//   - /api/v1/ route group protected by JWTAuth middleware
func New(v auth.TokenVerifier) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	_ = r.Group("/api/v1", middleware.JWTAuth(v))

	return r
}
