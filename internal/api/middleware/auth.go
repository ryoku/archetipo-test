package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/domain"
)

// JWTAuth returns a Gin middleware that validates a Bearer JWT using v.
// On success, stores *domain.UserIdentity in the Gin context under domain.IdentityContextKey.
// On failure, aborts with 401 {"error":"unauthorized"}.
func JWTAuth(v auth.TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		rawToken := strings.TrimPrefix(header, "Bearer ")
		identity, err := v.Verify(c.Request.Context(), rawToken)
		if err != nil || identity == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set(domain.IdentityContextKey, identity)
		c.Next()
	}
}
