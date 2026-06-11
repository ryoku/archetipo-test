package gitops

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/ryoku/kubegate/internal/domain"
)

var (
	// ErrHelmReleaseNotFound is returned when the HelmRelease file does not exist at the expected path.
	ErrHelmReleaseNotFound = errors.New("helmrelease not found")
	// ErrHelmReleaseParseFailed is returned when the HelmRelease file cannot be parsed.
	ErrHelmReleaseParseFailed = errors.New("helmrelease could not be parsed")
)

// HelmReleasePath returns the conventional gitops path for a product's HelmRelease file.
func HelmReleasePath(envSlug, productSlug string) string {
	return fmt.Sprintf("apps/%s/%s/%s-helmrelease.yaml", envSlug, productSlug, productSlug)
}

// DiscoverWorkloads parses a HelmRelease YAML document and returns every workload
// that has a spec.values.[workload].image.repository field. Workloads that do not
// have that field are silently skipped. Returns an empty slice (not an error) when
// spec.values is absent or contains no matching workloads.
func DiscoverWorkloads(data []byte) ([]domain.Workload, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty data", ErrHelmReleaseParseFailed)
	}

	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHelmReleaseParseFailed, err)
	}
	if len(file.Docs) == 0 {
		return nil, fmt.Errorf("%w: empty document", ErrHelmReleaseParseFailed)
	}

	root, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("%w: document root must be a YAML mapping", ErrHelmReleaseParseFailed)
	}

	valuesMap := specValuesMapping(root)
	if valuesMap == nil {
		return nil, nil
	}

	var workloads []domain.Workload
	for _, mv := range valuesMap.Values {
		name := mv.Key.String()
		workloadMap, ok := mv.Value.(*ast.MappingNode)
		if !ok {
			continue
		}

		repo := imageRepository(workloadMap)
		if repo == "" {
			continue
		}

		workloads = append(workloads, domain.Workload{
			Name:            name,
			ImageRepository: repo,
		})
	}
	return workloads, nil
}

// specValuesMapping navigates spec → values and returns the values mapping node,
// or nil if it is absent or not a mapping.
func specValuesMapping(root *ast.MappingNode) *ast.MappingNode {
	specMV := findMappingValue(root, "spec")
	if specMV == nil {
		return nil
	}
	specMap, ok := specMV.Value.(*ast.MappingNode)
	if !ok {
		return nil
	}

	valuesMV := findMappingValue(specMap, "values")
	if valuesMV == nil {
		return nil
	}
	if _, isNull := valuesMV.Value.(*ast.NullNode); isNull {
		return nil
	}
	valuesMap, ok := valuesMV.Value.(*ast.MappingNode)
	if !ok {
		return nil
	}
	return valuesMap
}

// imageRepository extracts spec.values.[workload].image.repository from a workload mapping node.
// Returns an empty string when the field is absent or not a string.
func imageRepository(workloadMap *ast.MappingNode) string {
	imageMV := findMappingValue(workloadMap, "image")
	if imageMV == nil {
		return ""
	}
	imageMap, ok := imageMV.Value.(*ast.MappingNode)
	if !ok {
		return ""
	}

	repoMV := findMappingValue(imageMap, "repository")
	if repoMV == nil {
		return ""
	}
	repoStr, ok := repoMV.Value.(*ast.StringNode)
	if !ok {
		return ""
	}
	return repoStr.Value
}
