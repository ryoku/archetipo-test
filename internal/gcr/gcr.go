package gcr

import (
	"context"
	"errors"
	"time"
)

// Tag represents a single image tag returned by the Artifact Registry.
type Tag struct {
	Name     string
	Digest   string
	PushedAt time.Time
}

// Lister lists image tags for an Artifact Registry image path.
type Lister interface {
	ListTags(ctx context.Context, imagePath, pageToken, filter string, pageSize int) (tags []Tag, nextPageToken string, err error)
}

// Typed errors returned by the GCR adapter.
var (
	ErrAuthFailure  = errors.New("authentication failure")
	ErrRepoNotFound = errors.New("repository not found")
	ErrRateLimit    = errors.New("API rate limit exceeded")
	ErrNetwork      = errors.New("network error")
)

type disabledLister struct{}

func (disabledLister) ListTags(_ context.Context, _, _, _ string, _ int) ([]Tag, string, error) {
	return nil, "", ErrAuthFailure
}

// Disabled returns a Lister that always returns ErrAuthFailure.
// Use it when GCP credentials are not available (e.g. local dev without ADC).
func Disabled() Lister { return disabledLister{} }
