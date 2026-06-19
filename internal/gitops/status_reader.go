package gitops

import "context"

// StatusReader reads the current deployment state from the gitops repo.
type StatusReader interface {
	ReadCurrentTags(ctx context.Context, productSlug, envSlug string) (map[string]string, error)
}
