package gitops

import (
	"errors"
	"testing"

	"github.com/ryoku/kubegate/internal/domain"
)

const discoverFixtureTwoWorkloads = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: platformapi
  namespace: flux-system
spec:
  values:
    main:
      image:
        repository: europe-west4-docker.pkg.dev/my-project/my-repo/platformapi-be
        tag: v1.0.0
    cron:
      image:
        repository: europe-west4-docker.pkg.dev/my-project/my-repo/platformapi-cron
        tag: v1.0.0
`

const discoverFixtureOneWorkloadNoRepo = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
spec:
  values:
    main:
      image:
        tag: v1.0.0
    worker:
      image:
        repository: us-docker.pkg.dev/proj/repo/worker
        tag: v2.0.0
`

const discoverFixtureMixedValues = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
spec:
  values:
    environment: dev
    replicaCount: 2
    main:
      image:
        repository: us-docker.pkg.dev/proj/repo/myapp
        tag: v1.0.0
`

const discoverFixtureNoSpecValues = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
spec:
  interval: 10m
`

const discoverFixtureNullSpecValues = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
spec:
  values: null
`

const discoverFixtureNoSpec = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
`

func TestDiscoverWorkloads_TwoWorkloads(t *testing.T) {
	workloads, err := DiscoverWorkloads([]byte(discoverFixtureTwoWorkloads))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d", len(workloads))
	}
	assertWorkload(t, workloads, "main", "europe-west4-docker.pkg.dev/my-project/my-repo/platformapi-be")
	assertWorkload(t, workloads, "cron", "europe-west4-docker.pkg.dev/my-project/my-repo/platformapi-cron")
}

func TestDiscoverWorkloads_SkipsWorkloadWithoutRepository(t *testing.T) {
	workloads, err := DiscoverWorkloads([]byte(discoverFixtureOneWorkloadNoRepo))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload (main without repo skipped), got %d", len(workloads))
	}
	assertWorkload(t, workloads, "worker", "us-docker.pkg.dev/proj/repo/worker")
}

func TestDiscoverWorkloads_SkipsNonMappingValues(t *testing.T) {
	workloads, err := DiscoverWorkloads([]byte(discoverFixtureMixedValues))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload (scalars and non-image mappings skipped), got %d", len(workloads))
	}
	assertWorkload(t, workloads, "main", "us-docker.pkg.dev/proj/repo/myapp")
}

func TestDiscoverWorkloads_NoSpecValues_ReturnsEmpty(t *testing.T) {
	workloads, err := DiscoverWorkloads([]byte(discoverFixtureNoSpecValues))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 0 {
		t.Errorf("expected 0 workloads, got %d", len(workloads))
	}
}

func TestDiscoverWorkloads_NullSpecValues_ReturnsEmpty(t *testing.T) {
	workloads, err := DiscoverWorkloads([]byte(discoverFixtureNullSpecValues))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 0 {
		t.Errorf("expected 0 workloads when spec.values is null, got %d", len(workloads))
	}
}

func TestDiscoverWorkloads_NoSpec_ReturnsEmpty(t *testing.T) {
	workloads, err := DiscoverWorkloads([]byte(discoverFixtureNoSpec))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 0 {
		t.Errorf("expected 0 workloads when spec key is absent, got %d", len(workloads))
	}
}

func TestDiscoverWorkloads_InvalidYAML_ReturnsParseError(t *testing.T) {
	_, err := DiscoverWorkloads([]byte("{{not valid yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !errors.Is(err, ErrHelmReleaseParseFailed) {
		t.Errorf("expected ErrHelmReleaseParseFailed, got %v", err)
	}
}

func TestDiscoverWorkloads_EmptyData_ReturnsParseError(t *testing.T) {
	_, err := DiscoverWorkloads(nil)
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
	if !errors.Is(err, ErrHelmReleaseParseFailed) {
		t.Errorf("expected ErrHelmReleaseParseFailed, got %v", err)
	}
}

func TestHelmReleasePath(t *testing.T) {
	tests := []struct {
		envSlug     string
		productSlug string
		want        string
	}{
		{"dev", "platformapi", "apps/dev/platformapi/platformapi-helmrelease.yaml"},
		{"integration", "my-app", "apps/integration/my-app/my-app-helmrelease.yaml"},
		{"production", "order-service", "apps/production/order-service/order-service-helmrelease.yaml"},
	}
	for _, tc := range tests {
		got := HelmReleasePath(tc.envSlug, tc.productSlug)
		if got != tc.want {
			t.Errorf("HelmReleasePath(%q, %q) = %q, want %q", tc.envSlug, tc.productSlug, got, tc.want)
		}
	}
}

func TestExtractCurrentTags_HappyPath(t *testing.T) {
	tags, err := ExtractCurrentTags([]byte(discoverFixtureTwoWorkloads))
	if err != nil {
		t.Fatalf("ExtractCurrentTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}
	if tags["main"] != "v1.0.0" {
		t.Errorf("main tag = %q, want %q", tags["main"], "v1.0.0")
	}
	if tags["cron"] != "v1.0.0" {
		t.Errorf("cron tag = %q, want %q", tags["cron"], "v1.0.0")
	}
}

func TestExtractCurrentTags_NoTagReturnsNA(t *testing.T) {
	const noTagFixture = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
spec:
  values:
    api:
      image:
        repository: gcr.io/proj/api
`
	tags, err := ExtractCurrentTags([]byte(noTagFixture))
	if err != nil {
		t.Fatalf("ExtractCurrentTags: %v", err)
	}
	if tags["api"] != TagMissing {
		t.Errorf("api tag = %q, want %q", tags["api"], TagMissing)
	}
}

func TestExtractCurrentTags_SkipsWorkloadsWithoutRepository(t *testing.T) {
	tags, err := ExtractCurrentTags([]byte(discoverFixtureOneWorkloadNoRepo))
	if err != nil {
		t.Fatalf("ExtractCurrentTags: %v", err)
	}
	if _, ok := tags["main"]; ok {
		t.Errorf("workload without repository should not appear in tags, got %q", tags["main"])
	}
	if tags["worker"] != "v2.0.0" {
		t.Errorf("worker tag = %q, want %q", tags["worker"], "v2.0.0")
	}
}

func TestExtractCurrentTags_NoValues_ReturnsEmptyMap(t *testing.T) {
	const noValues = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
spec:
  chart:
    spec:
      chart: my-chart
`
	tags, err := ExtractCurrentTags([]byte(noValues))
	if err != nil {
		t.Fatalf("ExtractCurrentTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected empty map, got %v", tags)
	}
}

func TestExtractCurrentTags_EmptyData_ReturnsParseError(t *testing.T) {
	_, err := ExtractCurrentTags(nil)
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
	if !errors.Is(err, ErrHelmReleaseParseFailed) {
		t.Errorf("expected ErrHelmReleaseParseFailed, got %v", err)
	}
}

func TestParseHelmRelease_NonMappingRoot_ReturnsParseError(t *testing.T) {
	// A syntactically valid YAML list at the document root triggers the "root must be a
	// mapping" guard in parseHelmRelease; both DiscoverWorkloads and ExtractCurrentTags
	// share this helper, so one fixture covers both entry points.
	listAtRoot := []byte("- foo\n- bar\n")
	_, err := DiscoverWorkloads(listAtRoot)
	if err == nil {
		t.Fatal("DiscoverWorkloads: expected error for non-mapping root, got nil")
	}
	if !errors.Is(err, ErrHelmReleaseParseFailed) {
		t.Errorf("DiscoverWorkloads: expected ErrHelmReleaseParseFailed, got %v", err)
	}

	_, err = ExtractCurrentTags(listAtRoot)
	if err == nil {
		t.Fatal("ExtractCurrentTags: expected error for non-mapping root, got nil")
	}
	if !errors.Is(err, ErrHelmReleaseParseFailed) {
		t.Errorf("ExtractCurrentTags: expected ErrHelmReleaseParseFailed, got %v", err)
	}
}

func assertWorkload(t *testing.T, workloads []domain.Workload, name, repo string) {
	t.Helper()
	for _, w := range workloads {
		if w.Name == name {
			if w.ImageRepository != repo {
				t.Errorf("workload %q: imageRepository = %q, want %q", name, w.ImageRepository, repo)
			}
			return
		}
	}
	t.Errorf("workload %q not found in result", name)
}
