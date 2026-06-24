package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/store"
)

// AdminHandlers bundles HTTP handlers for the /admin resource.
type AdminHandlers struct {
	store store.ProductStore
}

// NewAdminHandlers returns an AdminHandlers wired to the given store.
func NewAdminHandlers(s store.ProductStore) *AdminHandlers {
	return &AdminHandlers{store: s}
}

// adminProductResponse is the API representation of a product with aggregated stats.
type adminProductResponse struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Description      string  `json:"description"`
	CreatedAt        string  `json:"created_at"`
	EnvironmentCount int     `json:"environment_count"`
	LastDeployedAt   *string `json:"last_deployed_at"`
}

// GetAdminProducts handles GET /admin/products.
// Returns all non-archived products with aggregated stats.
// Requires DevOps Admin role (enforced by router middleware).
func (h *AdminHandlers) GetAdminProducts(c *gin.Context) {
	stats, err := h.store.ListWithStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	resp := make([]adminProductResponse, len(stats))
	for i, ps := range stats {
		r := adminProductResponse{
			ID:               ps.ID,
			Name:             ps.Name,
			Slug:             ps.Slug,
			Description:      ps.Description,
			CreatedAt:        ps.CreatedAt.Format(time.RFC3339),
			EnvironmentCount: ps.EnvironmentCount,
		}
		if ps.LastDeployedAt != nil {
			s := ps.LastDeployedAt.Format(time.RFC3339)
			r.LastDeployedAt = &s
		}
		resp[i] = r
	}
	c.JSON(http.StatusOK, resp)
}
