package domain

// Workload is a deployable unit discovered at runtime from a product's HelmRelease.
// Workloads are never persisted — they are read on demand from the gitops repo.
type Workload struct {
	Name            string
	ImageRepository string
}
