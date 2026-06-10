package domain

import "time"

// DeploymentLock holds the metadata of an in-progress deployment for a product-environment pair.
type DeploymentLock struct {
	ProductID   string
	EnvID       string
	LockHolder  string
	LockedSince time.Time
}
