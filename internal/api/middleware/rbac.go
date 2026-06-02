package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/domain"
)

// RequireRole returns a Gin middleware that enforces a minimum product-scoped role.
// It reads the product slug from the ":productSlug" URL parameter and checks
// UserIdentity.ProductRoles against the required role level.
//
// Response codes:
//   - 401 if no identity is present in the context (JWTAuth not applied upstream)
//   - 404 if the user has no role on the requested product (anti-enumeration)
//   - 403 if the user's role is below the required level
//
// DevOps Admin bypasses all per-product checks.
func RequireRole(need domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := identityFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if identity.IsDevOpsAdmin {
			c.Next()
			return
		}
		productSlug := c.Param("productSlug")
		have, exists := identity.ProductRoles[productSlug]
		if !exists {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if !domain.RoleAtLeast(have, need) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// RequireAdmin returns a Gin middleware that allows only DevOps Admins through.
// Returns 401 if no identity is in context, 403 for non-admin users.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := identityFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if !identity.IsDevOpsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// identityFromContext extracts *domain.UserIdentity from the Gin context.
func identityFromContext(c *gin.Context) (*domain.UserIdentity, bool) {
	v, exists := c.Get(domain.IdentityContextKey)
	if !exists {
		return nil, false
	}
	identity, ok := v.(*domain.UserIdentity)
	return identity, ok
}
