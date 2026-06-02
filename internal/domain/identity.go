package domain

// UserIdentity holds the authenticated user's claims extracted from a validated JWT.
type UserIdentity struct {
	Sub   string
	Email string
	Name  string
}

// IdentityContextKey is the Gin context key under which *UserIdentity is stored by JWTAuth.
const IdentityContextKey = "kubegate.user_identity"
