package auth

import (
	"context"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ryoku/kubegate/internal/domain"
)

// TokenVerifier is the seam used by JWTAuth middleware. Tests inject a mock.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*domain.UserIdentity, error)
}

// Verifier wraps the go-oidc IDTokenVerifier to implement TokenVerifier.
type Verifier struct {
	inner *gooidc.IDTokenVerifier
}

// NewVerifier fetches the OIDC discovery document at issuerURL and returns a Verifier.
// Fails fast if the provider is unreachable.
func NewVerifier(ctx context.Context, issuerURL string) (*Verifier, error) {
	provider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider discovery: %w", err)
	}
	v := provider.Verifier(&gooidc.Config{SkipClientIDCheck: true})
	return &Verifier{inner: v}, nil
}

// Verify parses and validates rawToken, returning the user identity on success.
func (v *Verifier) Verify(ctx context.Context, rawToken string) (*domain.UserIdentity, error) {
	token, err := v.inner.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}
	if claims.Sub == "" {
		return nil, fmt.Errorf("token missing required sub claim")
	}
	return &domain.UserIdentity{
		Sub:   claims.Sub,
		Email: claims.Email,
		Name:  claims.Name,
	}, nil
}
