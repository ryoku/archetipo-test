package domain

import (
	"fmt"
	"time"
)

// Environment represents a deployment target for a Product (e.g. dev, integration, production).
type Environment struct {
	ID          string
	ProductID   string
	Name        string
	Slug        string
	Type        string
	OverlayPath string
	CreatedAt   time.Time
}

// ValidateEnvironmentType returns an error if t is not one of the allowed environment types.
func ValidateEnvironmentType(t string) error {
	switch t {
	case "dev", "integration", "production":
		return nil
	default:
		return fmt.Errorf("type must be one of: dev, integration, production")
	}
}
