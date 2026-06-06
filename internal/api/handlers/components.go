package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// ComponentHandlers bundles HTTP handlers for /products/:productSlug/components.
type ComponentHandlers struct {
	productStore store.ProductStore
	compStore    store.ComponentStore
}

// NewComponentHandlers returns a ComponentHandlers wired to the given stores.
func NewComponentHandlers(ps store.ProductStore, cs store.ComponentStore) *ComponentHandlers {
	return &ComponentHandlers{productStore: ps, compStore: cs}
}

type createComponentRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	GCRImagePath string `json:"gcr_image_path"`
}

type componentResponse struct {
	ID           string `json:"id"`
	ProductID    string `json:"product_id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	GCRImagePath string `json:"gcr_image_path"`
	CreatedAt    string `json:"created_at"`
}

func toComponentResponse(c *domain.Component) componentResponse {
	return componentResponse{
		ID:           c.ID,
		ProductID:    c.ProductID,
		Name:         c.Name,
		Slug:         c.Slug,
		GCRImagePath: c.GCRImagePath,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
	}
}

// CreateComponent handles POST /api/v1/products/:productSlug/components.
func (h *ComponentHandlers) CreateComponent(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, slug)
	if !ok {
		return
	}

	var req createComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name is required"})
		return
	}
	if err := domain.ValidateSlug(req.Slug); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if req.GCRImagePath == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "gcr_image_path is required"})
		return
	}

	comp := &domain.Component{
		ProductID:    product.ID,
		Name:         req.Name,
		Slug:         req.Slug,
		GCRImagePath: req.GCRImagePath,
	}
	if err := h.compStore.Create(c.Request.Context(), comp); err != nil {
		if errors.Is(err, store.ErrComponentSlugConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "component slug already exists for this product"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusCreated, toComponentResponse(comp))
}

// ListComponents handles GET /api/v1/products/:productSlug/components.
func (h *ComponentHandlers) ListComponents(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, slug)
	if !ok {
		return
	}

	comps, err := h.compStore.ListByProduct(c.Request.Context(), product.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	resp := make([]componentResponse, len(comps))
	for i := range comps {
		resp[i] = toComponentResponse(&comps[i])
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteComponent handles DELETE /api/v1/products/:productSlug/components/:componentSlug.
func (h *ComponentHandlers) DeleteComponent(c *gin.Context) {
	productSlug := c.Param("productSlug")
	componentSlug := c.Param("componentSlug")

	if !checkProductAccess(c, productSlug) || !validateURLSlug(c, productSlug) {
		return
	}
	if err := domain.ValidateSlug(componentSlug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid component slug in URL"})
		return
	}

	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
		return
	}

	if err := h.compStore.Delete(c.Request.Context(), product.ID, componentSlug); err != nil {
		if errors.Is(err, store.ErrComponentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		if errors.Is(err, store.ErrComponentHasDeployments) {
			c.JSON(http.StatusConflict, gin.H{"error": "component has active deployment records"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.Status(http.StatusNoContent)
}
