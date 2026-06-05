package auth

import (
	"strings"

	"github.com/ryoku/kubegate/internal/domain"
)

// extractRoles parses a Keycloak realm_access.roles slice into product role assignments and the
// devops-admin flag. Roles not matching the kubegate: prefix, with unexpected structure, or with
// unknown role values are silently ignored.
func extractRoles(roles []string) (map[string]domain.Role, bool) {
	productRoles := make(map[string]domain.Role)
	isDevOpsAdmin := false
	for _, r := range roles {
		if !strings.HasPrefix(r, "kubegate:") {
			continue
		}
		parts := strings.Split(r, ":")
		switch len(parts) {
		case 2:
			if parts[1] == "devops-admin" {
				isDevOpsAdmin = true
			}
		case 3:
			if slug, role, ok := parseProductRole(parts); ok {
				productRoles[slug] = role
			}
		}
	}
	return productRoles, isDevOpsAdmin
}

// parseProductRole validates a 3-part kubegate claim of the form
// [kubegate, product-<slug>, <role>] and returns the slug and role.
// Returns ("", "", false) for claims that do not conform to the expected format.
func parseProductRole(parts []string) (string, domain.Role, bool) {
	if !strings.HasPrefix(parts[1], "product-") {
		return "", "", false
	}
	slug := strings.TrimPrefix(parts[1], "product-")
	if slug == "" {
		return "", "", false
	}
	role := domain.Role(parts[2])
	if role != domain.RoleEditor && role != domain.RoleViewer {
		return "", "", false
	}
	return slug, role, true
}
