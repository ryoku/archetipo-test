package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-git/v5"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// ReadCurrentTags clones the gitops repo, reads the HelmRelease for the given product and
// environment, and returns a map of workload name → currently deployed image tag.
// Returns "N/A" for workloads where the tag is absent. Returns a map with all workloads
// set to "N/A" when the HelmRelease does not exist (ErrHelmReleaseNotFound).
func (w *Writer) ReadCurrentTags(ctx context.Context, productSlug, envSlug string) (map[string]string, error) {
	tmpDir, err := os.MkdirTemp("", "kubegate-status-read-*")
	if err != nil {
		return nil, fmt.Errorf("gitops status: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // best-effort cleanup of temp clone; errors here are non-actionable

	auth, err := w.buildAuth()
	if err != nil {
		return nil, fmt.Errorf("gitops status: build auth: %w", err)
	}

	_, err = git.PlainCloneContext(ctx, tmpDir, false, &git.CloneOptions{
		URL:  w.cfg.RepoURL,
		Auth: auth,
	})
	if err != nil {
		return nil, fmt.Errorf("gitops status: clone: %w", err)
	}

	relPath := HelmReleasePath(envSlug, productSlug)
	absPath, err := securejoin.SecureJoin(tmpDir, relPath)
	if err != nil {
		return nil, fmt.Errorf("gitops status: unsafe helmrelease path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &HelmReleaseNotFoundError{Path: relPath}
		}
		return nil, fmt.Errorf("gitops status: read helmrelease: %w", err)
	}

	return parseCurrentTags(data)
}

// parseCurrentTags parses a HelmRelease YAML document and returns a map of
// workload name → image tag. Workloads without an image.tag are mapped to "N/A".
func parseCurrentTags(data []byte) (map[string]string, error) {
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
		return map[string]string{}, nil
	}

	tags := make(map[string]string, len(valuesMap.Values))
	for _, mv := range valuesMap.Values {
		name := mv.Key.String()
		workloadMap, ok := mv.Value.(*ast.MappingNode)
		if !ok {
			continue
		}
		tags[name] = imageTag(workloadMap)
	}
	return tags, nil
}

// imageTag extracts spec.values.[workload].image.tag from a workload mapping node.
// Returns "N/A" when the field is absent or not a string scalar.
func imageTag(workloadMap *ast.MappingNode) string {
	imageMV := findMappingValue(workloadMap, "image")
	if imageMV == nil {
		return "N/A"
	}
	imageMap, ok := imageMV.Value.(*ast.MappingNode)
	if !ok {
		return "N/A"
	}
	tagMV := findMappingValue(imageMap, "tag")
	if tagMV == nil {
		return "N/A"
	}
	tagStr, ok := tagMV.Value.(*ast.StringNode)
	if !ok {
		return "N/A"
	}
	return tagStr.Value
}
