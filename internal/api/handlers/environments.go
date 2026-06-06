package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// EnvironmentHandlers bundles HTTP handlers for /products/:productSlug/environments.
type EnvironmentHandlers struct {
	productStore store.ProductStore
	envStore     store.EnvironmentStore
}

// NewEnvironmentHandlers returns an EnvironmentHandlers wired to the given stores.
func NewEnvironmentHandlers(ps store.ProductStore, es store.EnvironmentStore) *EnvironmentHandlers {
	return &EnvironmentHandlers{productStore: ps, envStore: es}
}

type createEnvironmentRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	OverlayPath string `json:"overlay_path"`
}

type environmentResponse struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	OverlayPath string `json:"overlay_path"`
	CreatedAt   string `json:"created_at"`
}

func toEnvironmentResponse(e *domain.Environment) environmentResponse {
	return environmentResponse{
		ID:          e.ID,
		ProductID:   e.ProductID,
		Name:        e.Name,
		Type:        e.Type,
		OverlayPath: e.OverlayPath,
		CreatedAt:   e.CreatedAt.Format(time.RFC3339),
	}
}

// CreateEnvironment handles POST /api/v1/products/:productSlug/environments.
func (h *EnvironmentHandlers) CreateEnvironment(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	product, err := h.productStore.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	if product.ArchivedAt != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
		return
	}

	var req createEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name is required"})
		return
	}
	if err := domain.ValidateEnvironmentType(req.Type); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if req.OverlayPath == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "overlay_path is required"})
		return
	}
	if strings.HasPrefix(req.OverlayPath, "/") || strings.Contains(req.OverlayPath, "..") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "overlay_path must be a relative path"})
		return
	}

	env := &domain.Environment{
		ProductID:   product.ID,
		Name:        req.Name,
		Type:        req.Type,
		OverlayPath: req.OverlayPath,
	}
	if err := h.envStore.Create(c.Request.Context(), env); err != nil {
		if errors.Is(err, store.ErrEnvironmentNameConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "environment name already exists for this product"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusCreated, toEnvironmentResponse(env))
}

// ListEnvironments handles GET /api/v1/products/:productSlug/environments.
func (h *EnvironmentHandlers) ListEnvironments(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	product, err := h.productStore.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	if product.ArchivedAt != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
		return
	}

	envs, err := h.envStore.ListByProduct(c.Request.Context(), product.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	resp := make([]environmentResponse, len(envs))
	for i := range envs {
		resp[i] = toEnvironmentResponse(&envs[i])
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteEnvironment handles DELETE /api/v1/products/:productSlug/environments/:environmentID.
func (h *EnvironmentHandlers) DeleteEnvironment(c *gin.Context) {
	productSlug := c.Param("productSlug")
	environmentID := c.Param("environmentID")

	if !checkProductAccess(c, productSlug) || !validateURLSlug(c, productSlug) {
		return
	}
	if environmentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "environmentID is required"})
		return
	}

	product, err := h.productStore.GetBySlug(c.Request.Context(), productSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	if product.ArchivedAt != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
		return
	}

	if err := h.envStore.Delete(c.Request.Context(), product.ID, environmentID); err != nil {
		if errors.Is(err, store.ErrEnvironmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		if errors.Is(err, store.ErrEnvironmentHasDeployments) {
			c.JSON(http.StatusConflict, gin.H{"error": "environment has active deployment records"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.Status(http.StatusNoContent)
}
