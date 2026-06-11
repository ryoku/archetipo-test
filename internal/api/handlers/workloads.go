package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// WorkloadHandlers bundles HTTP handlers for workload discovery endpoints.
type WorkloadHandlers struct {
	productStore store.ProductStore
	envStore     store.EnvironmentStore
	reader       gitops.WorkloadReader
}

// NewWorkloadHandlers returns a WorkloadHandlers wired to the given stores and reader.
func NewWorkloadHandlers(ps store.ProductStore, es store.EnvironmentStore, r gitops.WorkloadReader) *WorkloadHandlers {
	return &WorkloadHandlers{productStore: ps, envStore: es, reader: r}
}

type workloadResponse struct {
	Name            string `json:"name"`
	ImageRepository string `json:"image_repository"`
}

// ListWorkloads handles GET /api/v1/products/:productSlug/environments/:environmentID/workloads.
func (h *WorkloadHandlers) ListWorkloads(c *gin.Context) {
	productSlug := c.Param("productSlug")
	environmentID := c.Param("environmentID")

	if !checkProductAccess(c, productSlug) || !validateURLSlug(c, productSlug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
		return
	}

	env, err := h.envStore.GetByID(c.Request.Context(), product.ID, environmentID)
	if err != nil {
		if errors.Is(err, store.ErrEnvironmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	workloads, err := h.reader.ListWorkloads(c.Request.Context(), product.Slug, env.Slug)
	if err != nil {
		switch {
		case errors.Is(err, gitops.ErrHelmReleaseNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, gitops.ErrHelmReleaseParseFailed):
			log.Printf("ListWorkloads product=%s env=%s: %v", productSlug, environmentID, err)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "HelmRelease file could not be parsed"})
		default:
			log.Printf("ListWorkloads product=%s env=%s: %v", productSlug, environmentID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		}
		return
	}

	resp := make([]workloadResponse, len(workloads))
	for i, w := range workloads {
		resp[i] = workloadResponse{Name: w.Name, ImageRepository: w.ImageRepository}
	}
	c.JSON(http.StatusOK, resp)
}
