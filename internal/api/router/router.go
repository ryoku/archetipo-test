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
//
// Each fn in registerRoutes receives the protected group to register handlers on.
// No routes are registered under /api/v1 until a handler spec calls router.New with its registration function.
func New(v auth.TokenVerifier, registerRoutes ...func(*gin.RouterGroup)) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1", middleware.JWTAuth(v))
	for _, fn := range registerRoutes {
		fn(api)
	}

	return r
}
