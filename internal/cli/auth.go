package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func cmdContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

const notLoggedInMsg = "Not logged in. Run `kubegate login`."

// loginRequired is the error returned when the user must authenticate.
// A named type is used so staticcheck ST1005 (error strings must not end with
// punctuation) does not flag the spec-mandated message format.
type loginRequired struct{}

func (loginRequired) Error() string { return notLoggedInMsg }

var errNotLoggedIn = loginRequired{}

// RequireAuthPreRun returns a cobra.PersistentPreRunE hook that enforces a
// valid token before any protected command executes. If the stored token is
// expired it attempts a silent refresh; on failure it returns the re-login
// prompt.
func RequireAuthPreRun(configDir string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		tok, err := ReadToken(configDir)
		if errors.Is(err, ErrTokenNotFound) {
			return errNotLoggedIn
		}
		if err != nil {
			return fmt.Errorf("reading stored token: %w", err)
		}

		if !IsExpired(tok) {
			return nil
		}

		if tok.RefreshToken == "" || tok.TokenURL == "" {
			return errNotLoggedIn
		}

		refreshed, err := refreshToken(cmdContext(cmd), tok)
		if err != nil {
			return errNotLoggedIn
		}

		return WriteToken(configDir, refreshed)
	}
}

func refreshToken(ctx context.Context, tok StoredToken) (StoredToken, error) {
	cfg := &oauth2.Config{
		ClientID: tok.ClientID,
		Endpoint: oauth2.Endpoint{
			TokenURL: tok.TokenURL,
		},
	}
	src := cfg.TokenSource(ctx, &oauth2.Token{
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	})
	newTok, err := src.Token()
	if err != nil {
		return StoredToken{}, err
	}

	refreshed := tok
	refreshed.AccessToken = newTok.AccessToken
	refreshed.TokenType = newTok.TokenType
	if newTok.RefreshToken != "" {
		refreshed.RefreshToken = newTok.RefreshToken
	}
	if !newTok.Expiry.IsZero() {
		refreshed.Expiry = newTok.Expiry
	} else {
		refreshed.Expiry = time.Now().Add(time.Hour)
	}
	return refreshed, nil
}
