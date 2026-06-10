package domain

import "testing"

func ptrStr(s string) *string { return &s }

type wantTagConvention struct {
	violation bool
	rejected  string
	applied   string
	err       bool
}

func TestCheckTagConvention(t *testing.T) {
	defaultRegex := `^v\d+\.\d+\.\d+$`

	tests := []struct {
		name         string
		tag          string
		envType      string
		productRegex *string
		defaultRegex string
		want         wantTagConvention
	}{
		{
			name:         "production env non-conforming tag returns violation",
			tag:          "latest",
			envType:      "production",
			productRegex: nil,
			defaultRegex: defaultRegex,
			want:         wantTagConvention{violation: true, rejected: "latest", applied: defaultRegex},
		},
		{
			name:         "production env conforming tag returns nil nil",
			tag:          "v1.2.3",
			envType:      "production",
			productRegex: nil,
			defaultRegex: defaultRegex,
		},
		{
			name:         "dev env non-conforming tag returns nil nil",
			tag:          "latest",
			envType:      "dev",
			productRegex: nil,
			defaultRegex: defaultRegex,
		},
		{
			name:         "integration env non-conforming tag returns nil nil",
			tag:          "latest",
			envType:      "integration",
			productRegex: nil,
			defaultRegex: defaultRegex,
		},
		{
			name:         "production env nil productRegex empty defaultRegex returns nil nil",
			tag:          "latest",
			envType:      "production",
			productRegex: nil,
			defaultRegex: "",
		},
		{
			name:         "product regex overrides default conforming tag returns nil nil",
			tag:          "hotfix-1",
			envType:      "production",
			productRegex: ptrStr(`^hotfix-\d+$`),
			defaultRegex: defaultRegex,
		},
		{
			name:         "product regex overrides default non-conforming tag returns violation with product regex",
			tag:          "v1.2.3",
			envType:      "production",
			productRegex: ptrStr(`^hotfix-\d+$`),
			defaultRegex: defaultRegex,
			want:         wantTagConvention{violation: true, rejected: "v1.2.3", applied: `^hotfix-\d+$`},
		},
		{
			name:         "invalid stored regex returns non-nil error",
			tag:          "v1.2.3",
			envType:      "production",
			productRegex: nil,
			defaultRegex: `[invalid`,
			want:         wantTagConvention{err: true},
		},
	}

	for _, tc := range tests {
		violation, err := CheckTagConvention(tc.tag, tc.envType, tc.productRegex, tc.defaultRegex)
		assertTagConventionResult(t, tc.name, tc.tag, tc.envType, tc.want, violation, err)
	}
}

func assertTagConventionResult(t *testing.T, caseName, tag, envType string, want wantTagConvention, v *TagConventionViolation, err error) {
	t.Helper()
	if (err != nil) != want.err {
		t.Errorf("[%s] error = %v, wantErr %v", caseName, err, want.err)
		return
	}
	if want.err {
		return
	}
	if !want.violation {
		if v != nil {
			t.Errorf("[%s] CheckTagConvention(%q, %q) = %+v, want nil violation", caseName, tag, envType, v)
		}
		return
	}
	if v == nil {
		t.Errorf("[%s] CheckTagConvention(%q, %q) = nil violation, want non-nil", caseName, tag, envType)
		return
	}
	if v.RejectedTag != want.rejected {
		t.Errorf("[%s] RejectedTag = %q, want %q", caseName, v.RejectedTag, want.rejected)
	}
	if v.AppliedRegex != want.applied {
		t.Errorf("[%s] AppliedRegex = %q, want %q", caseName, v.AppliedRegex, want.applied)
	}
}
