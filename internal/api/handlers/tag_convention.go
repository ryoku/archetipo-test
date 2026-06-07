package handlers

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/store"
)

// TagConventionHandlers bundles HTTP handlers for .../tag-convention endpoints.
type TagConventionHandlers struct {
	store        store.ProductStore
	defaultRegex string
}

// NewTagConventionHandlers returns a TagConventionHandlers wired to the given store and default regex.
func NewTagConventionHandlers(s store.ProductStore, defaultRegex string) *TagConventionHandlers {
	return &TagConventionHandlers{store: s, defaultRegex: defaultRegex}
}

// tagConventionResponse is the API representation of a tag convention.
type tagConventionResponse struct {
	Regex  string `json:"regex"`
	Source string `json:"source"` // "product" or "default"
}

// putTagConventionRequest is the body for PUT .../tag-convention.
type putTagConventionRequest struct {
	Regex *string `json:"regex"`
}

// GetTagConvention handles GET /api/v1/products/:productSlug/tag-convention.
// Returns the product-level override if set, otherwise the global default.
func (h *TagConventionHandlers) GetTagConvention(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	override, err := h.store.GetTagConvention(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	if override == nil {
		c.JSON(http.StatusOK, tagConventionResponse{Regex: h.defaultRegex, Source: "default"})
		return
	}
	c.JSON(http.StatusOK, tagConventionResponse{Regex: *override, Source: "product"})
}

// PutTagConvention handles PUT /api/v1/products/:productSlug/tag-convention.
// Accepts {"regex": "<pattern>"}, validates it as a compilable Go regex, and stores it.
// Returns 400 if regex is not compilable.
// Returns 422 if regex field is missing from body.
// Restricted to DevOps Admin and Editor roles (enforced by middleware upstream).
func (h *TagConventionHandlers) PutTagConvention(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	var req putTagConventionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Regex == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "regex is required"})
		return
	}
	if *req.Regex == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "regex must not be empty"})
		return
	}
	if len(*req.Regex) > 500 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "regex must not exceed 500 characters"})
		return
	}
	if _, err := regexp.Compile(*req.Regex); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid regex: not a valid Go regular expression"})
		return
	}

	if err := h.store.SetTagConvention(c.Request.Context(), slug, *req.Regex); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusOK, tagConventionResponse{Regex: *req.Regex, Source: "product"})
}
