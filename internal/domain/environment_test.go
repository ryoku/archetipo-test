package domain

import "testing"

func TestValidateEnvironmentType(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		// valid types
		{"dev", false},
		{"integration", false},
		{"production", false},
		// invalid types
		{"", true},
		{"staging", true},
		{"PRODUCTION", true},
	}
	for _, tc := range tests {
		err := ValidateEnvironmentType(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateEnvironmentType(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
		}
	}
}
