package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// NewLoginCmd returns the "kubegate login" command.
func NewLoginCmd(configDir string) *cobra.Command {
	var issuerURL, clientID string

	cmd := &cobra.Command{
		Use:          "login",
		Short:        "Authenticate via OIDC device authorization flow",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if issuerURL == "" {
				return errors.New("--issuer is required (or set KUBEGATE_ISSUER_URL)")
			}
			if clientID == "" {
				return errors.New("--client-id is required (or set KUBEGATE_CLIENT_ID)")
			}
			return runLogin(cmdContext(cmd), configDir, issuerURL, clientID)
		},
	}

	cmd.Flags().StringVar(&issuerURL, "issuer", envOrDefault("KUBEGATE_ISSUER_URL", ""), "OIDC issuer URL")
	cmd.Flags().StringVar(&clientID, "client-id", envOrDefault("KUBEGATE_CLIENT_ID", ""), "OIDC client ID")

	return cmd
}

func runLogin(ctx context.Context, configDir, issuerURL, clientID string) error {
	provider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return fmt.Errorf("OIDC discovery: %w", err)
	}

	var discovery struct {
		DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	}
	if err := provider.Claims(&discovery); err != nil {
		return fmt.Errorf("parse discovery document: %w", err)
	}
	if discovery.DeviceAuthorizationEndpoint == "" {
		return errors.New("OIDC provider does not advertise a device_authorization_endpoint")
	}

	endpoint := provider.Endpoint()
	endpoint.DeviceAuthURL = discovery.DeviceAuthorizationEndpoint

	cfg := &oauth2.Config{
		ClientID: clientID,
		Endpoint: endpoint,
		Scopes:   []string{gooidc.ScopeOpenID, "email", "profile", "offline_access"},
	}

	deviceResp, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return fmt.Errorf("device authorization request: %w", err)
	}

	if deviceResp.VerificationURIComplete != "" {
		_, _ = fmt.Fprintf(os.Stdout,
			"\nOpen the following URL in your browser:\n\n  %s\n\nWaiting for authorization...\n",
			deviceResp.VerificationURIComplete,
		)
	} else {
		_, _ = fmt.Fprintf(os.Stdout,
			"\nOpen the following URL in your browser:\n\n  %s\n\nAnd enter the code: %s\n\nWaiting for authorization...\n",
			deviceResp.VerificationURI, deviceResp.UserCode,
		)
	}

	token, err := cfg.DeviceAccessToken(ctx, deviceResp)
	if err != nil {
		return fmt.Errorf("awaiting device authorization: %w", err)
	}

	expiry := token.Expiry
	if expiry.IsZero() {
		expiry = time.Now().Add(time.Hour)
	}

	stored := StoredToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       expiry,
		IssuerURL:    issuerURL,
		ClientID:     clientID,
		TokenURL:     endpoint.TokenURL,
	}

	if err := WriteToken(configDir, stored); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "\nAuthentication successful. You are now logged in.")
	return nil
}

func envOrDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
