package domain

import "time"

const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// Deployment is an immutable record of a completed or failed deployment attempt.
type Deployment struct {
	ID            string
	ActorSub      string
	ProductID     string
	EnvironmentID string
	Workload      string
	Tag           string
	DeployedAt    time.Time
	CommitSHA     string
	Outcome       string
	ErrorMessage  string
}
