package domain

import "testing"

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		// valid slugs
		{"my-product", false},
		{"abc", false},
		{"product123", false},
		{"a1-b2-c3", false},
		// invalid slugs
		{"", true},
		{"My-Product", true},
		{"my product", true},
		{"my_product", true},
		{"-leading", true},
		{"trailing-", true},
		{"double--hyphen", true},
		{"ALLCAPS", true},
		{"with.dot", true},
	}
	for _, tc := range tests {
		err := ValidateSlug(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateSlug(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
		}
	}
}
