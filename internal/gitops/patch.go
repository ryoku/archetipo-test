package gitops

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// PatchInputField identifies which parameter failed validation in a PatchInputError.
type PatchInputField string

const (
	FieldImageName PatchInputField = "imageName"
	FieldNewTag    PatchInputField = "newTag"
)

// PatchInputError is returned when PatchImage receives invalid input.
type PatchInputError struct {
	Field  PatchInputField
	Reason string
}

func (e *PatchInputError) Error() string {
	return fmt.Sprintf("gitops patch: %s: %s", e.Field, e.Reason)
}

type imageEntry struct {
	Name    string `yaml:"name"`
	NewTag  string `yaml:"newTag,omitempty"`
	NewName string `yaml:"newName,omitempty"`
	// Digest is mutually exclusive with NewTag per the Kustomize images spec.
	// When NewTag is set, Digest must be cleared.
	Digest string `yaml:"digest,omitempty"`
}

// PatchImage returns a modified kustomization.yaml where the images entry for imageName
// has its newTag updated. All other keys and their ordering are preserved.
// If no entry exists for imageName, one is appended. If data is empty, a minimal overlay is created.
func PatchImage(data []byte, imageName, newTag string) ([]byte, error) {
	if imageName == "" {
		return nil, &PatchInputError{Field: FieldImageName, Reason: "must not be empty"}
	}
	if newTag == "" {
		return nil, &PatchInputError{Field: FieldNewTag, Reason: "must not be empty"}
	}

	if len(data) == 0 {
		stub := struct {
			Images []imageEntry `yaml:"images"`
		}{Images: []imageEntry{{Name: imageName, NewTag: newTag}}}
		return yaml.MarshalWithOptions(stub, yaml.IndentSequence(true))
	}

	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, fmt.Errorf("gitops patch: parse kustomization: %w", err)
	}
	if len(file.Docs) == 0 {
		return nil, fmt.Errorf("gitops patch: empty kustomization document")
	}

	rootMapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("gitops patch: kustomization.yaml root must be a YAML mapping")
	}

	imagesMV, currentImages, err := findImagesNode(rootMapping)
	if err != nil {
		return nil, err
	}

	updated := applyImageUpdate(currentImages, imageName, newTag)

	if imagesMV != nil {
		if err := replaceImagesNode(imagesMV, updated); err != nil {
			return nil, err
		}
	} else {
		if err := appendImagesNode(rootMapping, updated); err != nil {
			return nil, err
		}
	}

	return []byte(file.String()), nil
}

func findImagesNode(m *ast.MappingNode) (*ast.MappingValueNode, []imageEntry, error) {
	for _, mv := range m.Values {
		if mv.Key.String() != "images" {
			continue
		}
		if mv.Value == nil {
			// Explicit null images key — treat as empty list.
			return mv, nil, nil
		}
		var images []imageEntry
		if err := yaml.Unmarshal([]byte(mv.Value.String()), &images); err != nil {
			return nil, nil, fmt.Errorf("gitops patch: decode images: %w", err)
		}
		return mv, images, nil
	}
	return nil, nil, nil
}

func applyImageUpdate(images []imageEntry, imageName, newTag string) []imageEntry {
	for i, img := range images {
		if img.Name == imageName {
			images[i].NewTag = newTag
			images[i].Digest = "" // newTag and digest are mutually exclusive; clear digest when pinning by tag
			return images
		}
	}
	return append(images, imageEntry{Name: imageName, NewTag: newTag})
}

func replaceImagesNode(mv *ast.MappingValueNode, images []imageEntry) error {
	b, err := yaml.Marshal(images)
	if err != nil {
		return fmt.Errorf("gitops patch: marshal images: %w", err)
	}
	f, err := parser.ParseBytes(b, 0)
	if err != nil {
		return fmt.Errorf("gitops patch: parse updated images: %w", err)
	}
	return mv.Replace(f.Docs[0].Body)
}

func appendImagesNode(root *ast.MappingNode, images []imageEntry) error {
	// Wrap in a struct so IndentSequence applies to the images value, matching
	// the standard kustomization.yaml style of 2-space-indented sequence items.
	type snippet struct {
		Images []imageEntry `yaml:"images"`
	}
	b, err := yaml.MarshalWithOptions(snippet{Images: images}, yaml.IndentSequence(true))
	if err != nil {
		return fmt.Errorf("gitops patch: marshal images: %w", err)
	}
	f, err := parser.ParseBytes(b, 0)
	if err != nil {
		return fmt.Errorf("gitops patch: parse images snippet: %w", err)
	}
	m, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return fmt.Errorf("gitops patch: unexpected snippet AST structure")
	}
	root.Merge(m)
	return nil
}
