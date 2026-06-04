package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const tokenFileName = "token.json"

// StoredToken holds an OIDC token and the configuration needed to refresh it.
type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	IssuerURL    string    `json:"issuer_url"`
	ClientID     string    `json:"client_id"`
	TokenURL     string    `json:"token_url"`
}

// ErrTokenNotFound is returned when no token file exists.
var ErrTokenNotFound = errors.New("token file not found")

// ConfigDir returns ~/.config/kubegate, creating it if absent.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "kubegate")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ReadToken reads the stored token from dir/token.json.
// Returns ErrTokenNotFound when the file does not exist.
func ReadToken(dir string) (StoredToken, error) {
	path := filepath.Join(dir, tokenFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return StoredToken{}, ErrTokenNotFound
	}
	if err != nil {
		return StoredToken{}, err
	}
	var t StoredToken
	if err := json.Unmarshal(data, &t); err != nil {
		return StoredToken{}, fmt.Errorf("unmarshal token file: %w", err)
	}
	return t, nil
}

// WriteToken writes t to dir/token.json, creating dir if absent.
func WriteToken(dir string, t StoredToken) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, tokenFileName), data, 0o600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	return nil
}

// DeleteToken removes the token file from dir. It is not an error if the file
// does not exist.
func DeleteToken(dir string) error {
	err := os.Remove(filepath.Join(dir, tokenFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// IsExpired reports whether t's access token has expired (with a 10-second
// clock-skew buffer). A zero Expiry is treated as non-expired.
func IsExpired(t StoredToken) bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().After(t.Expiry.Add(-10 * time.Second))
}
