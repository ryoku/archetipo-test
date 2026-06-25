package domain

// Stats holds platform-level aggregate metrics scoped to a caller's accessible products.
type Stats struct {
	ProductCount     int `json:"product_count"`
	EnvironmentCount int `json:"environment_count"`
	ComponentCount   int `json:"component_count"`
	DeploymentsToday int `json:"deployments_today"`
}
