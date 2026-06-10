package gitops

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

const specValuesPrefix = "spec.values."

// HelmReleasePathError is returned when a required field is absent from the HelmRelease document.
type HelmReleasePathError struct {
	Path   string
	Reason string
}

func (e *HelmReleasePathError) Error() string {
	return fmt.Sprintf("helmrelease patch: %s: %s", e.Path, e.Reason)
}

// PatchHelmRelease returns the HelmRelease YAML with spec.values.[workload].image.tag
// updated to newTag. All other content, including FluxCD imagepolicy inline comments, is preserved.
// Only the first YAML document in data is patched; additional documents are passed through unchanged.
func PatchHelmRelease(data []byte, workload, newTag string) ([]byte, error) {
	if workload == "" {
		return nil, &PatchInputError{Field: FieldWorkload, Reason: "must not be empty"}
	}
	if newTag == "" {
		return nil, &PatchInputError{Field: FieldNewTag, Reason: "must not be empty"}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("helmrelease patch: input data is empty")
	}

	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("helmrelease patch: parse: %w", err)
	}
	if len(file.Docs) == 0 {
		return nil, fmt.Errorf("helmrelease patch: empty document")
	}

	root, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("helmrelease patch: document root must be a YAML mapping")
	}

	tagMV, err := walkHelmReleasePath(root, workload)
	if err != nil {
		return nil, err
	}

	sn, ok := tagMV.Value.(*ast.StringNode)
	if !ok {
		return nil, &HelmReleasePathError{
			Path:   specValuesPrefix + workload + ".image.tag",
			Reason: "value is not a scalar string",
		}
	}

	patchStringNode(sn, newTag)
	return []byte(file.String()), nil
}

// walkHelmReleasePath traverses spec → values → workload → image → tag
// and returns the MappingValueNode for the tag key.
func walkHelmReleasePath(root *ast.MappingNode, workload string) (*ast.MappingValueNode, error) {
	specMap, err := childMapping(root, "spec", "spec")
	if err != nil {
		return nil, err
	}
	valuesMap, err := childMapping(specMap, "values", "spec.values")
	if err != nil {
		return nil, err
	}
	workloadMap, err := childMapping(valuesMap, workload, specValuesPrefix+workload)
	if err != nil {
		return nil, err
	}
	imageMap, err := childMapping(workloadMap, "image", specValuesPrefix+workload+".image")
	if err != nil {
		return nil, err
	}

	tagMV := findMappingValue(imageMap, "tag")
	if tagMV == nil {
		return nil, &HelmReleasePathError{
			Path:   specValuesPrefix + workload + ".image.tag",
			Reason: "key not found",
		}
	}
	return tagMV, nil
}

// childMapping finds key in m and returns its value as a *ast.MappingNode.
func childMapping(m *ast.MappingNode, key, path string) (*ast.MappingNode, error) {
	mv := findMappingValue(m, key)
	if mv == nil {
		return nil, &HelmReleasePathError{Path: path, Reason: "key not found"}
	}
	if mv.Value == nil {
		return nil, &HelmReleasePathError{Path: path, Reason: "value is null"}
	}
	child, ok := mv.Value.(*ast.MappingNode)
	if !ok {
		return nil, &HelmReleasePathError{Path: path, Reason: "expected a YAML mapping"}
	}
	return child, nil
}

// findMappingValue returns the first MappingValueNode in m whose key matches name, or nil.
func findMappingValue(m *ast.MappingNode, name string) *ast.MappingValueNode {
	for _, mv := range m.Values {
		if mv.Key.GetToken().Value == name {
			return mv
		}
	}
	return nil
}

// patchStringNode updates n's value while preserving the original quote style.
func patchStringNode(n *ast.StringNode, newValue string) {
	origin := strings.TrimLeft(n.GetToken().Origin, " \t")
	var newOrigin string
	switch {
	case strings.HasPrefix(origin, "'"):
		newOrigin = "'" + newValue + "'"
	case strings.HasPrefix(origin, "\""):
		newOrigin = "\"" + newValue + "\""
	default:
		newOrigin = newValue
	}
	n.Value = newValue
	n.GetToken().Value = newValue
	n.GetToken().Origin = newOrigin
}
