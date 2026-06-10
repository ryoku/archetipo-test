package gcr

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// versionsProvider fetches one page of versions for a given AR package.
// Swappable in tests without mocking the full gRPC client.
type versionsProvider func(ctx context.Context, parent, pageToken string, pageSize int) ([]*artifactregistrypb.Version, string, error)

// Client implements Lister using the Artifact Registry gRPC API.
type Client struct {
	ar       *artifactregistry.Client
	provider versionsProvider
}

// NewClient creates a Client authenticated via Application Default Credentials.
// GOOGLE_APPLICATION_CREDENTIALS and Workload Identity on GKE are both honored.
func NewClient(ctx context.Context) (*Client, error) {
	ar, err := artifactregistry.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcr: create Artifact Registry client: %w", err)
	}
	return &Client{ar: ar, provider: newRealProvider(ar)}, nil
}

// Close releases the underlying gRPC connections held by the client.
func (c *Client) Close() error {
	return c.ar.Close()
}

func newRealProvider(ar *artifactregistry.Client) versionsProvider {
	return func(ctx context.Context, parent, pageToken string, pageSize int) ([]*artifactregistrypb.Version, string, error) {
		iter := ar.ListVersions(ctx, &artifactregistrypb.ListVersionsRequest{
			Parent:   parent,
			PageSize: int32(pageSize),
			View:     artifactregistrypb.VersionView_FULL,
		})
		pager := iterator.NewPager(iter, pageSize, pageToken)
		var versions []*artifactregistrypb.Version
		nextToken, err := pager.NextPage(&versions)
		if err != nil {
			return nil, "", err
		}
		return versions, nextToken, nil
	}
}

// ListTags returns image tags for the given Artifact Registry image path,
// sorted by push timestamp descending. pageSize is clamped to [1, 100];
// a zero value uses the default of 20.
func (c *Client) ListTags(ctx context.Context, imagePath, pageToken, filter string, pageSize int) ([]Tag, string, error) {
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	parsed, err := parseImagePath(imagePath)
	if err != nil {
		return nil, "", wrapSentinel(ErrRepoNotFound, err)
	}

	versions, nextToken, err := c.provider(ctx, parsed.resourceParent(), pageToken, pageSize)
	if err != nil {
		return nil, "", mapGRPCError(err)
	}

	tags := versionsToTags(versions)
	sort.SliceStable(tags, func(i, j int) bool {
		return tags[i].PushedAt.After(tags[j].PushedAt)
	})

	if filter != "" {
		filtered := tags[:0]
		for _, t := range tags {
			if strings.HasPrefix(t.Name, filter) {
				filtered = append(filtered, t)
			}
		}
		tags = filtered
		// Pagination is disabled when a filter is active: the next AR page token
		// was produced from a page that may have mostly unmatched items, so
		// returning it would cause "load more" to loop over mostly-empty pages.
		nextToken = ""
	}

	return tags, nextToken, nil
}

// versionsToTags expands AR Version objects into Tag values.
// One version can map to multiple tags (all sharing the same digest and push time).
func versionsToTags(versions []*artifactregistrypb.Version) []Tag {
	var tags []Tag
	for _, v := range versions {
		digest := extractLastSegment(v.GetName())
		pushedAtTime := createTime(v)

		for _, rt := range v.GetRelatedTags() {
			tags = append(tags, Tag{
				Name:     extractLastSegment(rt.GetName()),
				Digest:   digest,
				PushedAt: pushedAtTime,
			})
		}
		// Include untagged versions as digest-only entries
		if len(v.GetRelatedTags()) == 0 && digest != "" {
			tags = append(tags, Tag{
				Name:     digest,
				Digest:   digest,
				PushedAt: pushedAtTime,
			})
		}
	}
	return tags
}

// createTime safely extracts the push timestamp from a Version.
// Returns time.Time{} if the field is absent (malformed API response).
func createTime(v *artifactregistrypb.Version) time.Time {
	if t := v.GetCreateTime(); t != nil {
		return t.AsTime()
	}
	return time.Time{}
}

// extractLastSegment returns the last slash-delimited segment of a resource name.
func extractLastSegment(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// mapGRPCError maps a gRPC status error to one of the package's typed error sentinels.
func mapGRPCError(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		return wrapSentinel(ErrNetwork, err)
	}
	switch s.Code() {
	case codes.Unauthenticated, codes.PermissionDenied:
		return wrapSentinel(ErrAuthFailure, s.Message())
	case codes.NotFound, codes.InvalidArgument:
		return wrapSentinel(ErrRepoNotFound, s.Message())
	case codes.ResourceExhausted:
		return wrapSentinel(ErrRateLimit, s.Message())
	default:
		return wrapSentinel(ErrNetwork, err)
	}
}

// wrapSentinel wraps a typed sentinel error with additional detail.
func wrapSentinel(sentinel error, detail any) error {
	return fmt.Errorf("%w: %v", sentinel, detail)
}
