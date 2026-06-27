package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

const defaultHistoryPageSize = 20

// HistoryHandlers bundles the HTTP handlers for deployment history.
type HistoryHandlers struct {
	productStore    store.ProductStore
	deploymentStore store.DeploymentStore
}

// NewHistoryHandlers returns a HistoryHandlers wired to the given stores.
func NewHistoryHandlers(ps store.ProductStore, ds store.DeploymentStore) *HistoryHandlers {
	return &HistoryHandlers{productStore: ps, deploymentStore: ds}
}

type deploymentResponse struct {
	ID               string  `json:"id"`
	ActorDisplayName string  `json:"actor_display_name"`
	ComponentName    string  `json:"component_name"`
	EnvironmentName  string  `json:"environment_name"`
	Tag              string  `json:"tag"`
	DeployedAt       string  `json:"deployed_at"`
	CommitSHA        *string `json:"commit_sha"`
	Outcome          string  `json:"outcome"`
	ErrorMessage     *string `json:"error_message,omitempty"`
}

type historyResponse struct {
	Deployments []deploymentResponse `json:"deployments"`
	Page        int                  `json:"page"`
	PageSize    int                  `json:"page_size"`
	Total       int                  `json:"total"`
}

// ListByProduct handles GET /api/v1/products/:productSlug/deployments
func (h *HistoryHandlers) ListByProduct(c *gin.Context) {
	productSlug := c.Param("productSlug")
	if !validateURLSlug(c, productSlug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
		return
	}

	page := parsePageParam(c)
	deployments, total, err := h.deploymentStore.ListByProduct(c.Request.Context(), product.ID, page, defaultHistoryPageSize)
	if err != nil {
		log.Printf("ListByProduct product=%s page=%d: %v", productSlug, page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	c.JSON(http.StatusOK, historyResponse{
		Deployments: toDeploymentResponses(deployments),
		Page:        page,
		PageSize:    defaultHistoryPageSize,
		Total:       total,
	})
}

// ListAll handles GET /api/v1/admin/deployments
func (h *HistoryHandlers) ListAll(c *gin.Context) {
	page := parsePageParam(c)
	deployments, total, err := h.deploymentStore.ListAll(c.Request.Context(), page, defaultHistoryPageSize)
	if err != nil {
		log.Printf("ListAll page=%d: %v", page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	c.JSON(http.StatusOK, historyResponse{
		Deployments: toDeploymentResponses(deployments),
		Page:        page,
		PageSize:    defaultHistoryPageSize,
		Total:       total,
	})
}

func parsePageParam(c *gin.Context) int {
	p, err := strconv.Atoi(c.Query("page"))
	if err != nil || p < 1 {
		return 1
	}
	return p
}

func toDeploymentResponse(d domain.Deployment) deploymentResponse {
	return deploymentResponse{
		ID:               d.ID,
		ActorDisplayName: d.ActorDisplayName,
		ComponentName:    d.ComponentName,
		EnvironmentName:  d.EnvironmentName,
		Tag:              d.Tag,
		DeployedAt:       d.DeployedAt.UTC().Format(time.RFC3339),
		CommitSHA:        d.CommitSHA,
		Outcome:          string(d.Outcome),
		ErrorMessage:     d.ErrorMessage,
	}
}

func toDeploymentResponses(deployments []domain.Deployment) []deploymentResponse {
	result := make([]deploymentResponse, len(deployments))
	for i, d := range deployments {
		result[i] = toDeploymentResponse(d)
	}
	return result
}
