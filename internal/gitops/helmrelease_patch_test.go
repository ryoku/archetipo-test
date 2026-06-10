package gitops

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// helmReleaseFixture is a realistic HelmRelease modelled on mit-containers-gitops manifests.
// It has two workloads (main, cron) and FluxCD imagepolicy inline comments on the tag fields.
const helmReleaseFixture = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: platformapi
  namespace: flux-system
spec:
  interval: 10m
  releaseName: platformapi
  chart:
    spec:
      chart: ./apps/base/mmstandardapp/chart
  values:
    environment: dev
    main:
      enabled: true
      image:
        repository: europe-west4-docker.pkg.dev/mms-mit-infra/release-candidates/platformapi-be
        tag: 'v1.2.3' # {"$imagepolicy": "flux-system:platformapi-be-dev:tag"}
      service:
        port: 8080
    cron:
      enabled: true
      image:
        repository: europe-west4-docker.pkg.dev/mms-mit-infra/release-candidates/platformapi-be
        tag: 'v1.2.3' # {"$imagepolicy": "flux-system:platformapi-be-cron-dev:tag"}
`

// helmReleaseDoubleQuoted uses double-quoted tags to verify quote-style preservation.
const helmReleaseDoubleQuoted = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
      image:
        repository: gcr.io/proj/myapp
        tag: "v2.0.0"
`

// helmReleaseUnquoted uses unquoted tags.
const helmReleaseUnquoted = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: myapp
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
      image:
        repository: gcr.io/proj/myapp
        tag: v3.0.0
`

func TestPatchHelmRelease_InvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		workload string
		newTag   string
		field    PatchInputField
		reason   string
	}{
		{"empty workload", "", "v1.0.0", FieldWorkload, "must not be empty"},
		{"empty newTag", "main", "", FieldNewTag, "must not be empty"},
		{"tag starting with dot", "main", ".v1.0", FieldNewTag, "must be a valid OCI image tag"},
		{"tag with space", "main", "v1.0 patch", FieldNewTag, "must be a valid OCI image tag"},
		{"tag with single quote", "main", "v1.0'", FieldNewTag, "must be a valid OCI image tag"},
		{"tag with double quote", "main", `v1.0"`, FieldNewTag, "must be a valid OCI image tag"},
		{"tag with colon", "main", "v1.0:latest", FieldNewTag, "must be a valid OCI image tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PatchHelmRelease([]byte(helmReleaseFixture), tt.workload, tt.newTag)
			var pe *PatchInputError
			if !errors.As(err, &pe) {
				t.Fatalf("expected *PatchInputError, got %T: %v", err, err)
			}
			if pe.Field != tt.field {
				t.Errorf("Field = %q, want %q", pe.Field, tt.field)
			}
			if pe.Reason != tt.reason {
				t.Errorf("Reason = %q, want %q", pe.Reason, tt.reason)
			}
		})
	}
}

func TestPatchHelmRelease_EmptyInput(t *testing.T) {
	_, err := PatchHelmRelease(nil, "main", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error message = %q, want it to contain \"empty\"", err.Error())
	}
}

func TestPatchHelmRelease_NonMappingRoot(t *testing.T) {
	_, err := PatchHelmRelease([]byte("- foo\n- bar\n"), "main", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for non-mapping root, got nil")
	}
	if !strings.Contains(err.Error(), "document root must be a YAML mapping") {
		t.Errorf("error = %q, want it to contain \"document root must be a YAML mapping\"", err.Error())
	}
}

func TestPatchHelmRelease_SpecKeyMissing(t *testing.T) {
	input := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")
	_, err := PatchHelmRelease(input, "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if pe.Path != "spec" || pe.Reason != "key not found" {
		t.Errorf("Path=%q Reason=%q, want spec/key not found", pe.Path, pe.Reason)
	}
}

func TestPatchHelmRelease_ValuesKeyMissing(t *testing.T) {
	input := []byte("apiVersion: helm.toolkit.fluxcd.io/v2\nkind: HelmRelease\nspec:\n  interval: 10m\n")
	_, err := PatchHelmRelease(input, "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if pe.Path != "spec.values" {
		t.Errorf("Path=%q, want spec.values", pe.Path)
	}
}

func TestPatchHelmRelease_MalformedYAML(t *testing.T) {
	bad := []byte(":\tthis: is: not: valid: yaml\n  [broken")
	_, err := PatchHelmRelease(bad, "main", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "helmrelease patch") {
		t.Errorf("error message = %q, want prefix \"helmrelease patch\"", err.Error())
	}
}

func TestPatchHelmRelease_WorkloadNotFound(t *testing.T) {
	_, err := PatchHelmRelease([]byte(helmReleaseFixture), "ui", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Path, "ui") {
		t.Errorf("Path = %q, want it to contain the workload name %q", pe.Path, "ui")
	}
	if !strings.Contains(err.Error(), "ui") {
		t.Errorf("Error() = %q, want it to contain workload name %q", err.Error(), "ui")
	}
}

func TestPatchHelmRelease_TagNotScalar(t *testing.T) {
	fixture := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
      image:
        repository: gcr.io/proj/app
        tag:
          nested: value
`
	_, err := PatchHelmRelease([]byte(fixture), "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Reason, "scalar") {
		t.Errorf("Reason = %q, want it to mention \"scalar\"", pe.Reason)
	}
}

func TestPatchHelmRelease_ImageKeyMissing(t *testing.T) {
	fixture := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
      enabled: true
`
	_, err := PatchHelmRelease([]byte(fixture), "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Path, "image") {
		t.Errorf("Path = %q, want it to reference the missing \"image\" key", pe.Path)
	}
}

func TestPatchHelmRelease_TagKeyMissing(t *testing.T) {
	fixture := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
      image:
        repository: gcr.io/proj/app
`
	_, err := PatchHelmRelease([]byte(fixture), "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if !strings.Contains(pe.Path, "tag") {
		t.Errorf("Path = %q, want it to reference the missing \"tag\" key", pe.Path)
	}
	if pe.Reason != "key not found" {
		t.Errorf("Reason = %q, want \"key not found\"", pe.Reason)
	}
}

func TestPatchHelmRelease_NullTagValue(t *testing.T) {
	fixture := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
      image:
        repository: gcr.io/proj/app
        tag:
`
	_, err := PatchHelmRelease([]byte(fixture), "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if pe.Reason != "value is null" {
		t.Errorf("Reason = %q, want \"value is null\"", pe.Reason)
	}
	if !strings.Contains(pe.Path, "tag") {
		t.Errorf("Path = %q, want it to reference \"tag\"", pe.Path)
	}
}

func TestPatchHelmRelease_NullWorkloadValue(t *testing.T) {
	fixture := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: test
  namespace: flux-system
spec:
  interval: 10m
  values:
    main:
`
	_, err := PatchHelmRelease([]byte(fixture), "main", "v1.0.0")
	var pe *HelmReleasePathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *HelmReleasePathError, got %T: %v", err, err)
	}
	if pe.Reason != "value is null" {
		t.Errorf("Reason = %q, want \"value is null\"", pe.Reason)
	}
}

func TestPatchHelmRelease_SingleWorkload_HappyPath(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseFixture), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHelmTag(t, out, "main", "v9.9.9")
	assertRoundTrip(t, out)
}

func TestPatchHelmRelease_MultipleWorkloads_OnlyTargetChanged(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseFixture), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHelmTag(t, out, "main", "v9.9.9")
	// sibling workload must be unchanged
	assertHelmTag(t, out, "cron", "v1.2.3")
}

func TestPatchHelmRelease_FluxCDCommentPreserved(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseFixture), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `{"$imagepolicy": "flux-system:platformapi-be-dev:tag"}`) {
		t.Errorf("FluxCD imagepolicy comment missing from output:\n%s", out)
	}
	// the cron comment must be intact too
	if !strings.Contains(string(out), `{"$imagepolicy": "flux-system:platformapi-be-cron-dev:tag"}`) {
		t.Errorf("FluxCD imagepolicy cron comment missing from output:\n%s", out)
	}
}

func TestPatchHelmRelease_SingleQuoteStylePreserved(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseFixture), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "'v9.9.9'") {
		t.Errorf("single-quote style not preserved in output:\n%s", out)
	}
}

func TestPatchHelmRelease_DoubleQuoteStylePreserved(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseDoubleQuoted), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `"v9.9.9"`) {
		t.Errorf("double-quote style not preserved in output:\n%s", out)
	}
}

func TestPatchHelmRelease_UnquotedOriginalSafelyQuoted(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseUnquoted), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unquoted originals are written as single-quoted to prevent YAML scalar type
	// inference on re-parse (e.g. an unquoted "1.10" would reparse as float 1.1).
	if !strings.Contains(string(out), "'v9.9.9'") {
		t.Errorf("expected single-quoted output for unquoted original:\n%s", out)
	}
	assertHelmTag(t, out, "main", "v9.9.9")
}

func TestPatchHelmRelease_KeyOrderPreserved(t *testing.T) {
	out, err := PatchHelmRelease([]byte(helmReleaseFixture), "main", "v9.9.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	outStr := string(out)
	apiPos := strings.Index(outStr, "apiVersion")
	kindPos := strings.Index(outStr, "kind:")
	specPos := strings.Index(outStr, "spec:")
	if apiPos >= kindPos || kindPos >= specPos {
		t.Errorf("top-level key order not preserved:\n%s", outStr)
	}
	repoPos := strings.Index(outStr, "repository:")
	tagPos := strings.Index(outStr, "tag:")
	if repoPos >= tagPos {
		t.Errorf("image key order not preserved (repository must come before tag):\n%s", outStr)
	}
}

// assertHelmTag parses the output and verifies spec.values.[workload].image.tag.
// Uses map[string]interface{} because spec.values may contain mixed-type entries
// (e.g. "environment: dev" alongside workload mappings).
func assertHelmTag(t *testing.T, data []byte, workload, wantTag string) {
	t.Helper()
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, data)
	}
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec key not found in parsed output")
	}
	values, ok := spec["values"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.values key not found in parsed output")
	}
	wl, ok := values[workload].(map[string]interface{})
	if !ok {
		t.Fatalf("workload %q not found in parsed output", workload)
	}
	image, ok := wl["image"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec.values.%s.image not found in parsed output", workload)
	}
	tag, ok := image["tag"].(string)
	if !ok {
		t.Fatalf("spec.values.%s.image.tag not found in parsed output", workload)
	}
	if tag != wantTag {
		t.Errorf("spec.values.%s.image.tag = %q, want %q", workload, tag, wantTag)
	}
}

// assertRoundTrip verifies the output is valid YAML that parses without errors.
func assertRoundTrip(t *testing.T, data []byte) {
	t.Helper()
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("round-trip parse failed: %v\n%s", err, data)
	}
}
