package cli

import (
	"context"
	"fmt"
	"net/http"
)

// APIClient is a lightweight HTTP client for the KubeGate API.
type APIClient struct {
	baseURL    string
	tok        StoredToken
	httpClient *http.Client
}

// NewAPIClient returns an APIClient wired with the given base URL and stored token.
func NewAPIClient(baseURL string, tok StoredToken) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		tok:        tok,
		httpClient: &http.Client{},
	}
}

// Get performs a GET request to baseURL+path with a Bearer authorization header.
func (c *APIClient) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.tok.AccessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}
