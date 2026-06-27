package handlers

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/store"
)

// AdminHandlers bundles HTTP handlers for the /admin resource.
type AdminHandlers struct {
	store           store.ProductStore
	deploymentStore store.DeploymentStore
	staleDuration   time.Duration
}

// NewAdminHandlers returns an AdminHandlers wired to the given stores.
func NewAdminHandlers(s store.ProductStore, ds store.DeploymentStore, staleDuration time.Duration) *AdminHandlers {
	return &AdminHandlers{store: s, deploymentStore: ds, staleDuration: staleDuration}
}

// StaleDeploymentTimeout reads STALE_DEPLOYMENT_TIMEOUT_MINUTES from the environment.
// Falls back to 5 minutes when the variable is absent or invalid.
func StaleDeploymentTimeout() time.Duration {
	n, err := strconv.Atoi(os.Getenv("STALE_DEPLOYMENT_TIMEOUT_MINUTES"))
	if err != nil || n <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(n) * time.Minute
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

// activityResponse is the API representation of a single deployment activity entry.
type activityResponse struct {
	ID               string  `json:"id"`
	ActorDisplayName string  `json:"actor_display_name"`
	Tag              string  `json:"tag"`
	ComponentName    string  `json:"component_name"`
	ProductSlug      string  `json:"product_slug"`
	EnvironmentName  string  `json:"environment_name"`
	DeployedAt       string  `json:"deployed_at"`
	Outcome          string  `json:"outcome"`
	ErrorMessage     *string `json:"error_message,omitempty"`
}

// GetActivity handles GET /admin/activity.
// Returns the 10 most recent deployments across all products.
// Requires DevOps Admin role (enforced by router middleware).
func (h *AdminHandlers) GetActivity(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.deploymentStore.MarkStaleInProgress(ctx, h.staleDuration); err != nil {
		log.Printf("GetActivity: mark stale: %v", err)
		// non-fatal: proceed
	}
	deployments, err := h.deploymentStore.ListActivity(ctx, 10)
	if err != nil {
		log.Printf("GetActivity: list activity: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	resp := make([]activityResponse, len(deployments))
	for i, d := range deployments {
		r := activityResponse{
			ID:               d.ID,
			ActorDisplayName: d.ActorDisplayName,
			Tag:              d.Tag,
			ComponentName:    d.ComponentName,
			ProductSlug:      d.ProductSlug,
			EnvironmentName:  d.EnvironmentName,
			DeployedAt:       d.DeployedAt.UTC().Format(time.RFC3339),
			Outcome:          d.Outcome,
			ErrorMessage:     d.ErrorMessage,
		}
		resp[i] = r
	}
	c.JSON(http.StatusOK, resp)
}

// GetAdminProducts handles GET /admin/products.
// Returns all non-archived products with aggregated stats.
// Requires DevOps Admin role (enforced by router middleware).
func (h *AdminHandlers) GetAdminProducts(c *gin.Context) {
	stats, err := h.store.ListWithStats(c.Request.Context())
	if err != nil {
		log.Printf("GetAdminProducts: %v", err)
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
