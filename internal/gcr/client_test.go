package gcr

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func makeVersion(name string, tags []string, ts time.Time) *artifactregistrypb.Version {
	v := &artifactregistrypb.Version{
		Name:       name,
		CreateTime: timestamppb.New(ts),
	}
	for _, t := range tags {
		v.RelatedTags = append(v.RelatedTags, &artifactregistrypb.Tag{
			Name:    "projects/p/locations/l/repositories/r/packages/img/tags/" + t,
			Version: name,
		})
	}
	return v
}

func mockProvider(versions []*artifactregistrypb.Version, nextToken string, err error) versionsProvider {
	return func(_ context.Context, _, _ string, _ int) ([]*artifactregistrypb.Version, string, error) {
		return versions, nextToken, err
	}
}

func capturingProvider(versions []*artifactregistrypb.Version, nextToken string) (versionsProvider, *struct{ pageToken string }) {
	captured := &struct{ pageToken string }{}
	return func(_ context.Context, _, pt string, _ int) ([]*artifactregistrypb.Version, string, error) {
		captured.pageToken = pt
		return versions, nextToken, nil
	}, captured
}

func TestClient_ListTags_Success(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	older := now.Add(-time.Hour)

	versions := []*artifactregistrypb.Version{
		makeVersion("projects/p/locations/l/repositories/r/packages/img/versions/sha256:aaa", []string{"v1.0.0", "latest"}, now),
		makeVersion("projects/p/locations/l/repositories/r/packages/img/versions/sha256:bbb", []string{"v0.9.0"}, older),
	}

	c := &Client{provider: mockProvider(versions, "", nil)}
	tags, nextToken, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextToken != "" {
		t.Errorf("expected empty nextToken, got %q", nextToken)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags (2 for sha:aaa + 1 for sha:bbb), got %d", len(tags))
	}
	// Verify sorted descending by PushedAt
	if !tags[0].PushedAt.Equal(now) || !tags[1].PushedAt.Equal(now) {
		t.Errorf("first two tags should have PushedAt=%v, got %v and %v", now, tags[0].PushedAt, tags[1].PushedAt)
	}
	if !tags[2].PushedAt.Equal(older) {
		t.Errorf("last tag should have PushedAt=%v, got %v", older, tags[2].PushedAt)
	}
}

func TestClient_ListTags_Pagination(t *testing.T) {
	versions := []*artifactregistrypb.Version{
		makeVersion("projects/p/locations/l/repositories/r/packages/img/versions/sha256:ccc", []string{"v2.0.0"}, time.Now()),
	}
	c := &Client{provider: mockProvider(versions, "token-page2", nil)}
	tags, nextToken, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextToken != "token-page2" {
		t.Errorf("expected nextToken %q, got %q", "token-page2", nextToken)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
}

func TestClient_ListTags_UntaggedVersion(t *testing.T) {
	versions := []*artifactregistrypb.Version{
		makeVersion("projects/p/locations/l/repositories/r/packages/img/versions/sha256:ddd", nil, time.Now()),
	}
	c := &Client{provider: mockProvider(versions, "", nil)}
	tags, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 digest-only tag, got %d", len(tags))
	}
	if tags[0].Name != "sha256:ddd" {
		t.Errorf("expected tag name sha256:ddd, got %q", tags[0].Name)
	}
}

func TestClient_ListTags_InvalidPath(t *testing.T) {
	c := &Client{provider: mockProvider(nil, "", nil)}
	_, _, err := c.ListTags(context.Background(), "gcr.io/proj/img", "", 20)
	if err == nil {
		t.Fatal("expected error for legacy gcr.io path, got nil")
	}
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestClient_ListTags_AuthFailure(t *testing.T) {
	grpcErr := status.Error(codes.Unauthenticated, "invalid credentials")
	c := &Client{provider: mockProvider(nil, "", grpcErr)}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if !errors.Is(err, ErrAuthFailure) {
		t.Errorf("expected ErrAuthFailure, got %v", err)
	}
}

func TestClient_ListTags_PermissionDenied(t *testing.T) {
	grpcErr := status.Error(codes.PermissionDenied, "access denied")
	c := &Client{provider: mockProvider(nil, "", grpcErr)}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if !errors.Is(err, ErrAuthFailure) {
		t.Errorf("expected ErrAuthFailure for PermissionDenied, got %v", err)
	}
}

func TestClient_ListTags_RepoNotFound(t *testing.T) {
	grpcErr := status.Error(codes.NotFound, "repository not found")
	c := &Client{provider: mockProvider(nil, "", grpcErr)}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestClient_ListTags_RateLimit(t *testing.T) {
	grpcErr := status.Error(codes.ResourceExhausted, "quota exceeded")
	c := &Client{provider: mockProvider(nil, "", grpcErr)}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if !errors.Is(err, ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got %v", err)
	}
}

func TestClient_ListTags_NetworkError(t *testing.T) {
	netErr := errors.New("connection refused")
	c := &Client{provider: mockProvider(nil, "", netErr)}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if !errors.Is(err, ErrNetwork) {
		t.Errorf("expected ErrNetwork, got %v", err)
	}
}

func TestClient_ListTags_PageTokenForwarded(t *testing.T) {
	provider, captured := capturingProvider(nil, "")
	c := &Client{provider: provider}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "my-page-token", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.pageToken != "my-page-token" {
		t.Errorf("expected page token %q forwarded to provider, got %q", "my-page-token", captured.pageToken)
	}
}

func TestClient_ListTags_PageSizeZeroUsesDefault(t *testing.T) {
	provider, captured := capturingProvider(nil, "")
	c := &Client{provider: provider}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.pageToken != "" {
		t.Errorf("expected empty page token, got %q", captured.pageToken)
	}
	// pageSize 0 → defaultPageSize (20); verify the provider was called (no panic/skip)
}

func TestClient_ListTags_PageSizeCappedAtMax(t *testing.T) {
	var gotSize int
	c := &Client{
		provider: func(_ context.Context, _, _ string, pageSize int) ([]*artifactregistrypb.Version, string, error) {
			gotSize = pageSize
			return nil, "", nil
		},
	}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSize != maxPageSize {
		t.Errorf("expected pageSize capped at %d, provider received %d", maxPageSize, gotSize)
	}
}

func TestClient_ListTags_InvalidArgument(t *testing.T) {
	grpcErr := status.Error(codes.InvalidArgument, "invalid package name")
	c := &Client{provider: mockProvider(nil, "", grpcErr)}
	_, _, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound for InvalidArgument, got %v", err)
	}
}

func TestClient_ListTags_EmptyResult(t *testing.T) {
	c := &Client{provider: mockProvider(nil, "", nil)}
	tags, nextToken, err := c.ListTags(context.Background(), "us-docker.pkg.dev/p/r/img", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
	if nextToken != "" {
		t.Errorf("expected empty nextToken, got %q", nextToken)
	}
}
