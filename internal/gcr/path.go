package gcr

import (
	"fmt"
	"strings"
)

type parsedPath struct {
	project  string
	location string
	repo     string
	pkg      string
}

// parseImagePath parses an Artifact Registry Docker image path of the form
// {location}-docker.pkg.dev/{project}/{repository}/{image} into its components.
// Returns an error for legacy gcr.io paths and malformed inputs.
func parseImagePath(imagePath string) (parsedPath, error) {
	for _, prefix := range []string{"gcr.io/", "us.gcr.io/", "eu.gcr.io/", "asia.gcr.io/"} {
		if strings.HasPrefix(imagePath, prefix) {
			return parsedPath{}, fmt.Errorf("legacy Container Registry path %q is not supported; use an Artifact Registry path (*-docker.pkg.dev/...)", imagePath)
		}
	}

	slashIdx := strings.Index(imagePath, "/")
	if slashIdx == -1 {
		return parsedPath{}, fmt.Errorf("invalid image path %q: missing path segments", imagePath)
	}
	host := imagePath[:slashIdx]
	rest := imagePath[slashIdx+1:]

	const hostSuffix = "-docker.pkg.dev"
	if !strings.HasSuffix(host, hostSuffix) {
		return parsedPath{}, fmt.Errorf("invalid Artifact Registry host %q: expected format {location}-docker.pkg.dev", host)
	}
	location := strings.TrimSuffix(host, hostSuffix)
	if location == "" {
		return parsedPath{}, fmt.Errorf("cannot extract location from host %q", host)
	}

	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return parsedPath{}, fmt.Errorf("invalid image path %q: expected format {location}-docker.pkg.dev/{project}/{repo}/{image}", imagePath)
	}

	return parsedPath{
		project:  parts[0],
		location: location,
		repo:     parts[1],
		pkg:      parts[2],
	}, nil
}

// resourceParent builds the Artifact Registry API resource name for a package.
func (p parsedPath) resourceParent() string {
	return fmt.Sprintf("projects/%s/locations/%s/repositories/%s/packages/%s",
		p.project, p.location, p.repo, p.pkg)
}
