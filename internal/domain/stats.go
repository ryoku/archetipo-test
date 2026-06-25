package domain

// Stats holds platform-level aggregate metrics scoped to a caller's accessible products.
type Stats struct {
	ProductCount     int `json:"product_count"`
	EnvironmentCount int `json:"environment_count"`
	ComponentCount   int `json:"component_count"`
	// Deployments24h counts deployments in a rolling 24-hour window (not calendar-day "today").
	Deployments24h int `json:"deployments_last_24h"`
}
