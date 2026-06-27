package domain

import "time"

// DeploymentOutcome is the result of a deployment operation.
type DeploymentOutcome string

const (
	OutcomeSuccess    DeploymentOutcome = "success"
	OutcomeFailure    DeploymentOutcome = "failure"
	OutcomeInProgress DeploymentOutcome = "in_progress"
)

// Deployment records a single gitops apply operation.
type Deployment struct {
	ID               string
	ProductID        string
	ProductSlug      string // empty string if not populated by JOIN
	EnvironmentID    string
	ActorDisplayName string
	ComponentName    string
	EnvironmentName  string
	Tag              string
	DeployedAt       time.Time
	CommitSHA        *string           // nil until a commit is created — i.e. while in_progress, or if the deploy failed before committing
	Outcome          DeploymentOutcome // "success" | "failure" | "in_progress"
	ErrorMessage     *string           // non-nil only for failure outcomes; nil for success and in_progress
}
