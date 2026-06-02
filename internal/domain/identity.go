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
