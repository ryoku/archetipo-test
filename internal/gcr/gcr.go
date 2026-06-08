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
	ListTags(ctx context.Context, imagePath, pageToken string, pageSize int) (tags []Tag, nextPageToken string, err error)
}

// Typed errors returned by the GCR adapter.
var (
	ErrAuthFailure  = errors.New("authentication failure")
	ErrRepoNotFound = errors.New("repository not found")
	ErrRateLimit    = errors.New("API rate limit exceeded")
	ErrNetwork      = errors.New("network error")
)
