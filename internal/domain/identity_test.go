package domain

import "testing"

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		have Role
		need Role
		want bool
	}{
		{RoleViewer, RoleViewer, true},
		{RoleEditor, RoleEditor, true},
		{RoleEditor, RoleViewer, true},
		{RoleViewer, RoleEditor, false},
		// unknown roles must always fail closed
		{"", RoleViewer, false},
		{RoleViewer, "", false},
		{"", "", false},
	}
	for _, tc := range tests {
		got := RoleAtLeast(tc.have, tc.need)
		if got != tc.want {
			t.Errorf("RoleAtLeast(%q, %q) = %v, want %v", tc.have, tc.need, got, tc.want)
		}
	}
}
