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
			if !strings.HasPrefix(parts[1], "product-") {
				continue
			}
			slug := strings.TrimPrefix(parts[1], "product-")
			if slug == "" {
				continue
			}
			role := domain.Role(parts[2])
			if role != domain.RoleEditor && role != domain.RoleViewer {
				continue
			}
			productRoles[slug] = role
		}
	}
	return productRoles, isDevOpsAdmin
}
