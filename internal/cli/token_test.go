package cli_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryoku/kubegate/internal/cli"
)

func TestTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := cli.StoredToken{
		AccessToken:  "access-token",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
		IssuerURL:    "https://issuer.example.com",
		ClientID:     "kubegate-cli",
		TokenURL:     "https://issuer.example.com/protocol/openid-connect/token",
	}
	if err := cli.WriteToken(dir, want); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}
	got, err := cli.ReadToken(dir)
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if got.AccessToken != want.AccessToken {
		t.Errorf("AccessToken: got %q, want %q", got.AccessToken, want.AccessToken)
	}
	if got.RefreshToken != want.RefreshToken {
		t.Errorf("RefreshToken: got %q, want %q", got.RefreshToken, want.RefreshToken)
	}
	if got.IssuerURL != want.IssuerURL {
		t.Errorf("IssuerURL: got %q, want %q", got.IssuerURL, want.IssuerURL)
	}
	if got.TokenURL != want.TokenURL {
		t.Errorf("TokenURL: got %q, want %q", got.TokenURL, want.TokenURL)
	}
	if !got.Expiry.Equal(want.Expiry) {
		t.Errorf("Expiry: got %v, want %v", got.Expiry, want.Expiry)
	}
}

func TestReadTokenMissing(t *testing.T) {
	_, err := cli.ReadToken(t.TempDir())
	if !errors.Is(err, cli.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestDeleteToken(t *testing.T) {
	dir := t.TempDir()
	if err := cli.WriteToken(dir, cli.StoredToken{AccessToken: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := cli.DeleteToken(dir); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, err := cli.ReadToken(dir); !errors.Is(err, cli.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound after delete, got %v", err)
	}
	if err := cli.DeleteToken(dir); err != nil {
		t.Errorf("second DeleteToken should be no-op, got %v", err)
	}
}

func TestConfigDirCreatesKubegateConfigPath(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	dir, err := cli.ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}

	want := filepath.Join(configRoot, "kubegate")
	if dir != want {
		t.Fatalf("ConfigDir() = %q, want %q", dir, want)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("ConfigDir() path %q is not a directory", dir)
	}
}

func TestLogoutCmdRemovesStoredToken(t *testing.T) {
	dir := t.TempDir()
	if err := cli.WriteToken(dir, cli.StoredToken{AccessToken: "token"}); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = stdoutWriter
	t.Cleanup(func() { os.Stdout = oldStdout })

	cmd := cli.NewLogoutCmd(dir)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("stdoutWriter.Close: %v", err)
	}
	if _, err := io.ReadAll(stdoutReader); err != nil {
		t.Fatalf("ReadAll(stdoutReader): %v", err)
	}
	if _, err := cli.ReadToken(dir); !errors.Is(err, cli.ErrTokenNotFound) {
		t.Fatalf("expected token to be removed, got %v", err)
	}
}

func TestIsExpired(t *testing.T) {
	cases := []struct {
		name    string
		tok     cli.StoredToken
		expired bool
	}{
		{
			name:    "expired one hour ago",
			tok:     cli.StoredToken{Expiry: time.Now().Add(-time.Hour)},
			expired: true,
		},
		{
			name:    "valid for one hour",
			tok:     cli.StoredToken{Expiry: time.Now().Add(time.Hour)},
			expired: false,
		},
		{
			name:    "zero expiry treated as non-expired",
			tok:     cli.StoredToken{},
			expired: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cli.IsExpired(tc.tok); got != tc.expired {
				t.Errorf("IsExpired = %v, want %v", got, tc.expired)
			}
		})
	}
}
