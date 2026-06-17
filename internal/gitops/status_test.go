package gitops

import (
	"context"
	"errors"
	"testing"
)

const seedHelmReleaseWithTags = `apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: my-product
spec:
  values:
    api:
      image:
        repository: gcr.io/test-project/api
        tag: v1.2.3
    worker:
      image:
        repository: gcr.io/test-project/worker
        tag: v1.0.0
`

const helmReleaseNoTag = `apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: my-product
spec:
  values:
    api:
      image:
        repository: gcr.io/test-project/api
`

func TestParseCurrentTags_HappyPath(t *testing.T) {
	tags, err := parseCurrentTags([]byte(seedHelmReleaseWithTags))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tags["api"]; got != "v1.2.3" {
		t.Errorf("api tag: got %q, want %q", got, "v1.2.3")
	}
	if got := tags["worker"]; got != "v1.0.0" {
		t.Errorf("worker tag: got %q, want %q", got, "v1.0.0")
	}
}

func TestParseCurrentTags_MissingTag_ReturnsNA(t *testing.T) {
	tags, err := parseCurrentTags([]byte(helmReleaseNoTag))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tags["api"]; got != "N/A" {
		t.Errorf("api tag: got %q, want %q", got, "N/A")
	}
}

func TestParseCurrentTags_EmptyData(t *testing.T) {
	_, err := parseCurrentTags([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
}

func TestParseCurrentTags_NoSpecValues_ReturnsEmptyMap(t *testing.T) {
	noValues := `apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: my-product
spec:
  chart:
    spec:
      chart: my-chart
`
	tags, err := parseCurrentTags([]byte(noValues))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected empty map, got %v", tags)
	}
}

func TestWriter_ReadCurrentTags_HappyPath(t *testing.T) {
	const path = "apps/production/my-product/my-product-helmrelease.yaml"
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		path: seedHelmReleaseWithTags,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tags, err := w.ReadCurrentTags(context.Background(), "my-product", "production")
	if err != nil {
		t.Fatalf("ReadCurrentTags: %v", err)
	}
	if got := tags["api"]; got != "v1.2.3" {
		t.Errorf("api tag: got %q, want %q", got, "v1.2.3")
	}
	if got := tags["worker"]; got != "v1.0.0" {
		t.Errorf("worker tag: got %q, want %q", got, "v1.0.0")
	}
}

func TestWriter_ReadCurrentTags_HelmReleaseNotFound(t *testing.T) {
	const path = "apps/production/my-product/my-product-helmrelease.yaml"
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		path: seedHelmReleaseWithTags,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = w.ReadCurrentTags(context.Background(), "other-product", "production")
	if err == nil {
		t.Fatal("expected error for missing HelmRelease, got nil")
	}
	if !errors.Is(err, ErrHelmReleaseNotFound) {
		t.Errorf("expected ErrHelmReleaseNotFound, got: %v", err)
	}
}
