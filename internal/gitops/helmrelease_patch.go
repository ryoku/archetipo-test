package gitops

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

const specValuesPrefix = "spec.values."

// HelmReleasePathError is returned when traversal of the HelmRelease document fails —
// a required key is missing, its value is null, or the value is not the expected type.
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
	if !isValidOCITag(newTag) {
		return nil, &PatchInputError{Field: FieldNewTag, Reason: "must be a valid OCI image tag"}
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

	tagPath := specValuesPrefix + workload + ".image.tag"
	sn, ok := tagMV.Value.(*ast.StringNode)
	if !ok {
		if _, isNull := tagMV.Value.(*ast.NullNode); isNull {
			return nil, &HelmReleasePathError{Path: tagPath, Reason: "value is null"}
		}
		return nil, &HelmReleasePathError{Path: tagPath, Reason: "value is not a scalar string"}
	}

	if err := patchStringNode(sn, newTag); err != nil {
		return nil, err
	}
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
// Returns HelmReleasePathError if the key is absent, its value is null, or it is not a mapping.
func childMapping(m *ast.MappingNode, key, path string) (*ast.MappingNode, error) {
	mv := findMappingValue(m, key)
	if mv == nil {
		return nil, &HelmReleasePathError{Path: path, Reason: "key not found"}
	}
	if _, ok := mv.Value.(*ast.NullNode); ok {
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
		if mv.Key.String() == name {
			return mv
		}
	}
	return nil
}

// patchStringNode updates n's value while preserving the original quote style.
// All three fields must be updated: n.Value is the Go string, Token.Value is used by the
// serialiser, and Token.Origin is the raw bytes written to output. Unquoted originals are
// re-serialised as single-quoted to prevent YAML scalar type inference (e.g. "1.10" → float 1.1).
func patchStringNode(n *ast.StringNode, newValue string) error {
	tok := n.GetToken()
	if tok == nil {
		return fmt.Errorf("helmrelease patch: internal error: string node has no token")
	}
	origin := strings.TrimLeft(tok.Origin, " \t")
	var newOrigin string
	switch {
	case strings.HasPrefix(origin, "'"):
		newOrigin = "'" + newValue + "'"
	case strings.HasPrefix(origin, "\""):
		newOrigin = "\"" + newValue + "\""
	default:
		// Re-serialise unquoted values as single-quoted to prevent YAML scalar type
		// inference on re-parse (e.g. an unquoted "1.10" would reparse as float 1.1).
		// StringNode.String() dispatches on tok.Type, so the type must change too.
		// Callers guarantee newValue is a valid OCI tag (no single quotes).
		newOrigin = "'" + newValue + "'"
		tok.Type = token.SingleQuoteType
	}
	n.Value = newValue
	tok.Value = newValue
	tok.Origin = newOrigin
	return nil
}

// isValidOCITag reports whether tag is a valid OCI image tag:
// non-empty, at most 128 characters, starting with [a-zA-Z0-9_] and
// containing only [a-zA-Z0-9_.\-].
func isValidOCITag(tag string) bool {
	if len(tag) == 0 || len(tag) > 128 {
		return false
	}
	for i, c := range tag {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			// valid in all positions
		case (c == '.' || c == '-') && i > 0:
			// valid after first character
		default:
			return false
		}
	}
	return true
}
