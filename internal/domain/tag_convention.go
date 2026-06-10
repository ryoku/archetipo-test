package domain

import (
	"fmt"
	"regexp"
)

// TagConventionViolation is returned when a tag does not match the required production convention.
type TagConventionViolation struct {
	RejectedTag  string
	AppliedRegex string
}

// CheckTagConvention validates tag against the convention for production environments.
// Returns nil when envType is not "production", or when no regex is configured.
// Returns *TagConventionViolation when tag does not match the applicable regex.
// Returns a non-nil error when the stored regex is syntactically invalid.
func CheckTagConvention(tag, envType string, productRegex *string, defaultRegex string) (*TagConventionViolation, error) {
	if envType != "production" {
		return nil, nil
	}

	appliedRegex := defaultRegex
	if productRegex != nil {
		appliedRegex = *productRegex
	}
	if appliedRegex == "" {
		return nil, nil
	}

	re, err := regexp.Compile(appliedRegex)
	if err != nil {
		return nil, fmt.Errorf("tag convention regex is invalid: %w", err)
	}
	if !re.MatchString(tag) {
		return &TagConventionViolation{RejectedTag: tag, AppliedRegex: appliedRegex}, nil
	}
	return nil, nil
}
