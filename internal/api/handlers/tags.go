package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/store"
)

// TagHandlers bundles HTTP handlers for /products/:productSlug/components/:componentSlug/tags.
type TagHandlers struct {
	productStore store.ProductStore
	compStore    store.ComponentStore
	lister       gcr.Lister
}

// NewTagHandlers returns a TagHandlers wired to the given stores and lister.
func NewTagHandlers(ps store.ProductStore, cs store.ComponentStore, l gcr.Lister) *TagHandlers {
	return &TagHandlers{productStore: ps, compStore: cs, lister: l}
}

type tagResponse struct {
	Name     string `json:"name"`
	Digest   string `json:"digest"`
	PushedAt string `json:"pushed_at"`
}

type listTagsResponse struct {
	Tags          []tagResponse `json:"tags"`
	NextPageToken string        `json:"next_page_token,omitempty"`
}

// ListTags handles GET /api/v1/products/:productSlug/components/:componentSlug/tags.
func (h *TagHandlers) ListTags(c *gin.Context) {
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

	comp, err := h.compStore.GetBySlug(c.Request.Context(), product.ID, componentSlug)
	if err != nil {
		if errors.Is(err, store.ErrComponentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		log.Printf("GetBySlug product=%s component=%s: %v", productSlug, componentSlug, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	if comp.GCRImagePath == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "component has no GCR image path configured"})
		return
	}

	pageToken := c.Query("page_token")
	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		n, parseErr := strconv.Atoi(ps)
		if parseErr != nil || n <= 0 || n > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page_size must be between 1 and 100"})
			return
		}
		pageSize = n
	}

	filter := c.Query("filter")
	if len(filter) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filter must be 200 characters or fewer"})
		return
	}
	tags, nextToken, err := h.lister.ListTags(c.Request.Context(), comp.GCRImagePath, pageToken, filter, pageSize)
	if err != nil {
		h.replyListerError(c, productSlug, componentSlug, err)
		return
	}

	resp := listTagsResponse{
		Tags:          make([]tagResponse, len(tags)),
		NextPageToken: nextToken,
	}
	for i, t := range tags {
		resp.Tags[i] = tagResponse{
			Name:     t.Name,
			Digest:   t.Digest,
			PushedAt: t.PushedAt.Format(time.RFC3339),
		}
	}
	c.JSON(http.StatusOK, resp)
}

// replyListerError maps a gcr.Lister error to an HTTP response.
// ErrRepoNotFound → 404, ErrRateLimit → 429.
// Auth failures and network errors are logged server-side and both return 502
// to avoid exposing internal failure details to clients.
func (h *TagHandlers) replyListerError(c *gin.Context, productSlug, componentSlug string, err error) {
	switch {
	case errors.Is(err, gcr.ErrRepoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "image repository not found"})
	case errors.Is(err, gcr.ErrRateLimit):
		log.Printf("ListTags %s/%s: Artifact Registry rate limit: %v", productSlug, componentSlug, err)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Artifact Registry rate limit exceeded"})
	case errors.Is(err, gcr.ErrAuthFailure):
		log.Printf("ListTags %s/%s: authentication failure: %v", productSlug, componentSlug, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "error contacting Artifact Registry"})
	default:
		log.Printf("ListTags %s/%s: %v", productSlug, componentSlug, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "error contacting Artifact Registry"})
	}
}
