package domain

import (
	"fmt"
	"regexp"
	"time"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Product represents a managed software product in KubeGate.
type Product struct {
	ID          string
	Name        string
	Slug        string
	Description        string
	TagConventionRegex *string
	ArchivedAt         *time.Time
	CreatedAt   time.Time
}

// ValidateSlug returns an error if s does not match the slug pattern
// (lowercase alphanumeric words joined by single hyphens, no leading/trailing hyphens).
func ValidateSlug(s string) error {
	if s == "" {
		return fmt.Errorf("slug is required")
	}
	if !slugPattern.MatchString(s) {
		return fmt.Errorf("slug must be lowercase alphanumeric words separated by hyphens (e.g. my-product)")
	}
	return nil
}
