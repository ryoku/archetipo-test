package gitops

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// testKustomization mirrors the Kustomize root mapping for assertion purposes.
type testKustomization struct {
	Images []imageEntry   `yaml:"images"`
	Extra  map[string]any `yaml:",inline"`
}

func TestPatchImage_InvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		newTag    string
		field     PatchInputField
		reason    string
	}{
		{"empty imageName", "", "v1.0.0", FieldImageName, "must not be empty"},
		{"empty newTag", "gcr.io/proj/svc", "", FieldNewTag, "must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PatchImage(nil, tt.imageName, tt.newTag)
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
			if got := pe.Error(); !strings.Contains(got, string(tt.field)) {
				t.Errorf("Error() = %q, want it to contain %q", got, tt.field)
			}
		})
	}
}

func TestPatchImage_EmptyInput(t *testing.T) {
	out, err := PatchImage(nil, "gcr.io/proj/svc", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc", "v1.0.0")
}

func TestPatchImage_SingleImageOverlay(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
images:
  - name: gcr.io/proj/svc
    newTag: v1.0.0
`)
	out, err := PatchImage(input, "gcr.io/proj/svc", "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc", "v1.2.0")
	assertPreservedKeys(t, k, "apiVersion", "kind", "resources")
}

func TestPatchImage_KeyOrderPreserved(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
images:
  - name: gcr.io/proj/svc
    newTag: v1.0.0
`)
	out, err := PatchImage(input, "gcr.io/proj/svc", "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	outStr := string(out)
	apiPos := strings.Index(outStr, "apiVersion")
	kindPos := strings.Index(outStr, "kind:")
	resPos := strings.Index(outStr, "resources:")
	imgPos := strings.Index(outStr, "images:")
	if apiPos >= kindPos || kindPos >= resPos || resPos >= imgPos {
		t.Errorf("key order not preserved:\n%s", outStr)
	}
}

func TestPatchImage_MultiImageOverlay_OnlyTargetChanged(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: gcr.io/proj/svc-a
    newTag: v1.0.0
  - name: gcr.io/proj/svc-b
    newTag: v2.0.0
  - name: gcr.io/proj/svc-c
    newTag: v3.0.0
`)
	out, err := PatchImage(input, "gcr.io/proj/svc-b", "v2.5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 3 {
		t.Fatalf("len(images) = %d, want 3", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc-a", "v1.0.0")
	assertImage(t, k.Images[1], "gcr.io/proj/svc-b", "v2.5.0")
	assertImage(t, k.Images[2], "gcr.io/proj/svc-c", "v3.0.0")
}

func TestPatchImage_NoExistingImagesEntry(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
`)
	out, err := PatchImage(input, "gcr.io/proj/svc", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc", "v1.0.0")
	assertPreservedKeys(t, k, "apiVersion", "kind", "resources")
}

func TestPatchImage_DigestPinnedEntry_ClearsDigest(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: gcr.io/proj/svc
    digest: sha256:abc123
`)
	out, err := PatchImage(input, "gcr.io/proj/svc", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc", "v1.0.0")
	if k.Images[0].Digest != "" {
		t.Errorf("Digest = %q, want empty (newTag and digest must not coexist)", k.Images[0].Digest)
	}
}

func TestPatchImage_NewNamePreservedOnSiblingEntries(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: gcr.io/proj/svc-a
    newName: private.registry.io/proj/svc-a
    newTag: v1.0.0
  - name: gcr.io/proj/svc-b
    newTag: v2.0.0
`)
	out, err := PatchImage(input, "gcr.io/proj/svc-b", "v2.5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 2 {
		t.Fatalf("len(images) = %d, want 2", len(k.Images))
	}
	if k.Images[0].NewName != "private.registry.io/proj/svc-a" {
		t.Errorf("sibling newName = %q, want %q", k.Images[0].NewName, "private.registry.io/proj/svc-a")
	}
	assertImage(t, k.Images[1], "gcr.io/proj/svc-b", "v2.5.0")
}

func TestPatchImage_NonMappingRoot(t *testing.T) {
	// A YAML sequence at the root is not a valid kustomization.yaml.
	input := []byte("- name: foo\n  newTag: v1\n")
	_, err := PatchImage(input, "gcr.io/proj/svc", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for non-mapping YAML root, got nil")
	}
}

func TestPatchImage_NewNamePreservedOnTargetEntry(t *testing.T) {
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: gcr.io/proj/svc
    newName: private.registry.io/proj/svc
    newTag: v1.0.0
`)
	out, err := PatchImage(input, "gcr.io/proj/svc", "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc", "v1.2.0")
	if k.Images[0].NewName != "private.registry.io/proj/svc" {
		t.Errorf("newName = %q, want %q", k.Images[0].NewName, "private.registry.io/proj/svc")
	}
}

func TestPatchImage_NullImagesValue(t *testing.T) {
	// Kustomization with an explicit null images key should append a new entry.
	input := []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
`)
	out, err := PatchImage(input, "gcr.io/proj/svc", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := parseKustomization(t, out)
	if len(k.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(k.Images))
	}
	assertImage(t, k.Images[0], "gcr.io/proj/svc", "v1.0.0")
}

func parseKustomization(t *testing.T, data []byte) testKustomization {
	t.Helper()
	var k testKustomization
	if err := yaml.Unmarshal(data, &k); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, data)
	}
	return k
}

func assertImage(t *testing.T, got imageEntry, wantName, wantTag string) {
	t.Helper()
	if got.Name != wantName {
		t.Errorf("image.Name = %q, want %q", got.Name, wantName)
	}
	if got.NewTag != wantTag {
		t.Errorf("image.NewTag = %q, want %q", got.NewTag, wantTag)
	}
}

func assertPreservedKeys(t *testing.T, k testKustomization, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := k.Extra[key]; !ok {
			t.Errorf("key %q missing from output YAML", key)
		}
	}
}
