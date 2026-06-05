package domain

import "time"

// Component represents a deployable unit within a Product, identified by a GCR image repository.
type Component struct {
	ID           string
	ProductID    string
	Name         string
	Slug         string
	GCRImagePath string
	CreatedAt    time.Time
}
