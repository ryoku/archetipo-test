package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
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
	Name string `json:"name"`
	Slug string `json:"slug"`
	Type string `json:"type"`
}

type environmentResponse struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Type       string `json:"type"`
	GitopsPath string `json:"gitops_path"`
	CreatedAt  string `json:"created_at"`
}

func toEnvironmentResponse(e *domain.Environment, productSlug string) environmentResponse {
	return environmentResponse{
		ID:         e.ID,
		ProductID:  e.ProductID,
		Name:       e.Name,
		Slug:       e.Slug,
		Type:       e.Type,
		GitopsPath: gitops.HelmReleasePath(e.Slug, productSlug),
		CreatedAt:  e.CreatedAt.Format(time.RFC3339),
	}
}

// CreateEnvironment handles POST /api/v1/products/:productSlug/environments.
func (h *EnvironmentHandlers) CreateEnvironment(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, slug)
	if !ok {
		return
	}

	var req createEnvironmentRequest
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name is required"})
		return
	}
	if err := domain.ValidateSlug(req.Slug); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "slug: " + err.Error()})
		return
	}
	if err := domain.ValidateEnvironmentType(req.Type); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	env := &domain.Environment{
		ProductID: product.ID,
		Name:      req.Name,
		Slug:      req.Slug,
		Type:      req.Type,
	}
	if err := h.envStore.Create(c.Request.Context(), env); err != nil {
		if errors.Is(err, store.ErrEnvironmentNameConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "environment name already exists for this product"})
			return
		}
		if errors.Is(err, store.ErrEnvironmentSlugConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "environment slug already exists for this product"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusCreated, toEnvironmentResponse(env, product.Slug))
}

// ListEnvironments handles GET /api/v1/products/:productSlug/environments.
func (h *EnvironmentHandlers) ListEnvironments(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, slug)
	if !ok {
		return
	}

	envs, err := h.envStore.ListByProduct(c.Request.Context(), product.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	resp := make([]environmentResponse, len(envs))
	for i := range envs {
		resp[i] = toEnvironmentResponse(&envs[i], product.Slug)
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

	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
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
