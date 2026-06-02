package domain

// Role is the access level a user holds within a product.
type Role = string

const (
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// UserIdentity holds the authenticated user's claims extracted from a validated JWT.
type UserIdentity struct {
	Sub          string
	Email        string
	Name         string
	ProductRoles map[string]Role
	IsDevOpsAdmin bool
}

// IdentityContextKey is the Gin context key under which *UserIdentity is stored by JWTAuth.
const IdentityContextKey = "kubegate.user_identity"

// RoleAtLeast reports whether have meets or exceeds the need role level.
// Role ordering: viewer < editor. Returns false if either role is unknown.
func RoleAtLeast(have, need Role) bool {
	lh := roleOrder(have)
	ln := roleOrder(need)
	if lh == 0 || ln == 0 {
		return false
	}
	return lh >= ln
}

// roleOrder returns the numeric rank of r, or 0 for unknown roles.
func roleOrder(r Role) int {
	switch r {
	case RoleViewer:
		return 1
	case RoleEditor:
		return 2
	default:
		return 0
	}
}
