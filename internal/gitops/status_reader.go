package gitops

import "context"

type StatusReader interface {
	ReadCurrentTags(ctx context.Context, productSlug, envSlug string) (map[string]string, error)
}
