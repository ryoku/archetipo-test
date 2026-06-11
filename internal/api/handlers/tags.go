package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// TagHandlers bundles HTTP handlers for workload tag listing.
type TagHandlers struct {
	productStore store.ProductStore
	envStore     store.EnvironmentStore
	reader       gitops.WorkloadReader
	lister       gcr.Lister
}

// NewTagHandlers returns a TagHandlers wired to the given stores, reader and lister.
func NewTagHandlers(ps store.ProductStore, es store.EnvironmentStore, r gitops.WorkloadReader, l gcr.Lister) *TagHandlers {
	return &TagHandlers{productStore: ps, envStore: es, reader: r, lister: l}
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

// ListTags handles GET /api/v1/products/:productSlug/environments/:environmentID/workloads/:workload/tags.
func (h *TagHandlers) ListTags(c *gin.Context) {
	productSlug := c.Param("productSlug")
	environmentID := c.Param("environmentID")
	workloadName := c.Param("workload")

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

	imageRepo, ok := h.resolveWorkloadImageRepo(c, productSlug, environmentID, product.Slug, env.Slug, workloadName)
	if !ok {
		return
	}

	pageToken, pageSize, filter, ok := parseTagListParams(c)
	if !ok {
		return
	}

	tags, nextToken, err := h.lister.ListTags(c.Request.Context(), imageRepo, pageToken, filter, pageSize)
	if err != nil {
		h.replyListerError(c, productSlug, workloadName, err)
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

// resolveWorkloadImageRepo calls ListWorkloads and returns the image repository for the named workload.
func (h *TagHandlers) resolveWorkloadImageRepo(c *gin.Context, productSlug, environmentID, productSlugForGit, envSlug, workloadName string) (string, bool) {
	workloads, err := h.reader.ListWorkloads(c.Request.Context(), productSlugForGit, envSlug)
	if err != nil {
		switch {
		case errors.Is(err, gitops.ErrHelmReleaseNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, gitops.ErrHelmReleaseParseFailed):
			log.Printf("ListTags product=%s env=%s workload=%s: parse error: %v", productSlug, environmentID, workloadName, err)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "HelmRelease file could not be parsed"})
		default:
			log.Printf("ListTags product=%s env=%s workload=%s: %v", productSlug, environmentID, workloadName, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		}
		return "", false
	}

	for _, w := range workloads {
		if w.Name == workloadName {
			return w.ImageRepository, true
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "workload not found in HelmRelease"})
	return "", false
}

// parseTagListParams extracts and validates page_token, page_size and filter query params.
func parseTagListParams(c *gin.Context) (pageToken string, pageSize int, filter string, ok bool) {
	pageToken = c.Query("page_token")
	pageSize = 20
	if ps := c.Query("page_size"); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n <= 0 || n > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page_size must be between 1 and 100"})
			return "", 0, "", false
		}
		pageSize = n
	}
	filter = c.Query("filter")
	if len(filter) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filter must be 200 characters or fewer"})
		return "", 0, "", false
	}
	return pageToken, pageSize, filter, true
}

func (h *TagHandlers) replyListerError(c *gin.Context, productSlug, workload string, err error) {
	switch {
	case errors.Is(err, gcr.ErrRepoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "image repository not found"})
	case errors.Is(err, gcr.ErrRateLimit):
		log.Printf("ListTags %s/%s: Artifact Registry rate limit: %v", productSlug, workload, err)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Artifact Registry rate limit exceeded"})
	case errors.Is(err, gcr.ErrAuthFailure):
		log.Printf("ListTags %s/%s: authentication failure: %v", productSlug, workload, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "error contacting Artifact Registry"})
	default:
		log.Printf("ListTags %s/%s: %v", productSlug, workload, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "error contacting Artifact Registry"})
	}
}
