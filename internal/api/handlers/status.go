package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// StatusReader reads the currently deployed image tags from the gitops repo.
// Defined here as a local interface to follow the dependency direction of the handlers package.
type StatusReader interface {
	ReadCurrentTags(ctx context.Context, productSlug, envSlug string) (map[string]string, error)
}

// statusResponse is the API response shape for GET /products/:productSlug/status.
type statusResponse struct {
	Workloads map[string]map[string]string `json:"workloads"`
	FetchedAt string                       `json:"fetched_at"`
	Stale     bool                         `json:"stale"`
}

type statusCacheEntry struct {
	result    statusResponse
	fetchedAt time.Time
}

type statusCache struct {
	mu   sync.Mutex
	data map[string]statusCacheEntry
	ttl  time.Duration
}

func newStatusCache() *statusCache {
	ttl := 60 * time.Second
	if n, err := strconv.Atoi(os.Getenv("STATUS_CACHE_TTL_SECONDS")); err == nil && n > 0 {
		ttl = time.Duration(n) * time.Second
	}
	return &statusCache{
		data: make(map[string]statusCacheEntry),
		ttl:  ttl,
	}
}

// get returns a cached entry and whether it was found and fresh.
func (c *statusCache) get(key string) (statusResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.data[key]
	if !ok {
		return statusResponse{}, false
	}
	if time.Since(entry.fetchedAt) >= c.ttl {
		delete(c.data, key)
		return statusResponse{}, false
	}
	return entry.result, true
}

// set stores a response in the cache.
func (c *statusCache) set(key string, result statusResponse, fetchedAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = statusCacheEntry{result: result, fetchedAt: fetchedAt}
}

// StatusHandlers bundles HTTP handlers for the deployment status endpoint.
type StatusHandlers struct {
	productStore store.ProductStore
	envStore     store.EnvironmentStore
	reader       StatusReader
	cache        *statusCache
}

// NewStatusHandlers returns a StatusHandlers wired to the given stores and reader.
func NewStatusHandlers(ps store.ProductStore, es store.EnvironmentStore, r StatusReader) *StatusHandlers {
	return &StatusHandlers{
		productStore: ps,
		envStore:     es,
		reader:       r,
		cache:        newStatusCache(),
	}
}

// GetStatus handles GET /api/v1/products/:productSlug/status.
// Returns a matrix of workload × environment → current image tag.
func (h *StatusHandlers) GetStatus(c *gin.Context) {
	productSlug := c.Param("productSlug")

	if !checkProductAccess(c, productSlug) || !validateURLSlug(c, productSlug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
		return
	}

	if cached, hit := h.cache.get(product.Slug); hit {
		c.JSON(http.StatusOK, cached)
		return
	}

	envs, err := h.envStore.ListByProduct(c.Request.Context(), product.ID)
	if err != nil {
		log.Printf("GetStatus product=%s: list environments: %v", productSlug, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	workloads := make(map[string]map[string]string)
	for _, env := range envs {
		tags, readErr := h.reader.ReadCurrentTags(c.Request.Context(), product.Slug, env.Slug)
		if readErr != nil {
			if errors.Is(readErr, gitops.ErrHelmReleaseNotFound) {
				// No HelmRelease for this env — all workloads are N/A; continue to next env.
				continue
			}
			if errors.Is(readErr, gitops.ErrGitOpsNotConfigured) {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "deployment status is not available on this server"})
				return
			}
			log.Printf("GetStatus product=%s env=%s: %v", productSlug, env.Slug, readErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
			return
		}
		for workloadName, tag := range tags {
			if workloads[workloadName] == nil {
				workloads[workloadName] = make(map[string]string)
			}
			workloads[workloadName][env.Slug] = tag
		}
	}

	fetchedAt := time.Now().UTC()
	resp := statusResponse{
		Workloads: workloads,
		FetchedAt: fetchedAt.Format(time.RFC3339),
		Stale:     false,
	}
	h.cache.set(product.Slug, resp, fetchedAt)
	c.JSON(http.StatusOK, resp)
}
