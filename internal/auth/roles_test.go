package auth

import (
	"testing"

	"github.com/ryoku/kubegate/internal/domain"
)

func TestExtractRoles(t *testing.T) {
	tests := []struct {
		name              string
		roles             []string
		wantProductRoles  map[string]domain.Role
		wantIsDevOpsAdmin bool
	}{
		{
			name:              "nil slice yields empty results",
			roles:             nil,
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "empty slice yields empty results",
			roles:             []string{},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "non-kubegate roles are silently ignored",
			roles:             []string{"realm:admin", "other:role", "openid"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "devops-admin role sets flag",
			roles:             []string{"kubegate:devops-admin"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: true,
		},
		{
			name:              "editor role for a product slug",
			roles:             []string{"kubegate:product-foo:editor"},
			wantProductRoles:  map[string]domain.Role{"foo": domain.RoleEditor},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "viewer role for a product slug",
			roles:             []string{"kubegate:product-bar:viewer"},
			wantProductRoles:  map[string]domain.Role{"bar": domain.RoleViewer},
			wantIsDevOpsAdmin: false,
		},
		{
			name: "editor and viewer for different slugs",
			roles: []string{
				"kubegate:product-alpha:editor",
				"kubegate:product-beta:viewer",
			},
			wantProductRoles:  map[string]domain.Role{"alpha": domain.RoleEditor, "beta": domain.RoleViewer},
			wantIsDevOpsAdmin: false,
		},
		{
			name: "mixed kubegate and non-kubegate roles",
			roles: []string{
				"kubegate:product-foo:editor",
				"kubegate:devops-admin",
				"realm:user",
				"other:ignored",
			},
			wantProductRoles:  map[string]domain.Role{"foo": domain.RoleEditor},
			wantIsDevOpsAdmin: true,
		},
		{
			name:              "malformed: kubegate: with empty second segment ignored",
			roles:             []string{"kubegate:"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "malformed: kubegate:product-foo without role segment ignored",
			roles:             []string{"kubegate:product-foo"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "malformed: unknown role value ignored",
			roles:             []string{"kubegate:product-foo:superadmin"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "malformed: too many segments ignored",
			roles:             []string{"kubegate:product-foo:editor:extra"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "malformed: product- prefix with empty slug ignored",
			roles:             []string{"kubegate:product-:editor"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
		{
			name:              "malformed: kubegate: prefix but not product- pattern and not devops-admin ignored",
			roles:             []string{"kubegate:unknown-prefix:editor"},
			wantProductRoles:  map[string]domain.Role{},
			wantIsDevOpsAdmin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoles, gotAdmin := extractRoles(tt.roles)
			if gotAdmin != tt.wantIsDevOpsAdmin {
				t.Errorf("IsDevOpsAdmin = %v, want %v", gotAdmin, tt.wantIsDevOpsAdmin)
			}
			if len(gotRoles) != len(tt.wantProductRoles) {
				t.Errorf("ProductRoles len = %d, want %d; got %v", len(gotRoles), len(tt.wantProductRoles), gotRoles)
				return
			}
			for slug, wantRole := range tt.wantProductRoles {
				if gotRole, ok := gotRoles[slug]; !ok || gotRole != wantRole {
					t.Errorf("ProductRoles[%q] = %q (ok=%v), want %q", slug, gotRole, ok, wantRole)
				}
			}
		})
	}
}
