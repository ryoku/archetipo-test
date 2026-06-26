package domain

import (
	"fmt"
	"regexp"
	"time"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Product represents a managed software product in KubeGate.
type Product struct {
	ID                 string
	Name               string
	Slug               string
	Description        string
	TagConventionRegex *string
	ArchivedAt         *time.Time
	CreatedAt          time.Time
	// LastDeployedAt and HasProductionEnv are derived aggregates populated only
	// by List and ListWithStats. They are zero-valued (nil / false) on a Product
	// obtained any other way (GetBySlug, GetByID, Create, Update), so callers
	// must not treat them as authoritative outside the list paths.
	LastDeployedAt   *time.Time
	HasProductionEnv bool
}

// ProductStats embeds Product and adds the per-product environment count for the
// admin dashboard. Last-deploy and production-env aggregates now live on Product
// itself (populated by both List and ListWithStats).
type ProductStats struct {
	Product
	EnvironmentCount int
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
