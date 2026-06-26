package domain

import "time"

const (
	OutcomeSuccess    = "success"
	OutcomeFailure    = "failure"
	OutcomeInProgress = "in_progress"
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
	CommitSHA        *string // nil when the deploy failed before a commit was created
	Outcome          string  // "success" | "failure" | "in_progress"
	ErrorMessage     *string // non-nil only when Outcome is "failure"
}
