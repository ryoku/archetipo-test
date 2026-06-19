package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"

	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

// statusFetchTimeout caps the gitops clone+read so a hung git server cannot block indefinitely.
const statusFetchTimeout = 30 * time.Second

// StatusResponse is the shape returned by GET /products/:productSlug/status.
type StatusResponse struct {
	Workloads map[string]map[string]string `json:"workloads"`
	FetchedAt string                       `json:"fetched_at"`
	Stale     bool                         `json:"stale"`
}

type statusCacheEntry struct {
	result    StatusResponse
	fetchedAt time.Time
}

type statusCache struct {
	mu   sync.Mutex
	data map[string]statusCacheEntry
	ttl  time.Duration
}

// statusFetchError carries an HTTP status code so the handler can write the right response.
type statusFetchError struct {
	code    int
	message string
}

func (e *statusFetchError) Error() string { return e.message }

// StatusHandlers bundles HTTP handlers for the deployment status endpoints.
type StatusHandlers struct {
	productStore store.ProductStore
	envStore     store.EnvironmentStore
	reader       gitops.StatusReader
	cache        *statusCache
	sf           singleflight.Group
}

// NewStatusHandlers returns a StatusHandlers wired to the given stores, reader, and TTL from env.
func NewStatusHandlers(ps store.ProductStore, es store.EnvironmentStore, r gitops.StatusReader) *StatusHandlers {
	return newStatusHandlers(ps, es, r, statusCacheTTL())
}

// NewStatusHandlersWithTTL is identical to NewStatusHandlers but accepts an explicit TTL.
// Use in tests to control cache expiry without relying on environment variables.
func NewStatusHandlersWithTTL(ps store.ProductStore, es store.EnvironmentStore, r gitops.StatusReader, ttl time.Duration) *StatusHandlers {
	return newStatusHandlers(ps, es, r, ttl)
}

func newStatusHandlers(ps store.ProductStore, es store.EnvironmentStore, r gitops.StatusReader, ttl time.Duration) *StatusHandlers {
	return &StatusHandlers{
		productStore: ps,
		envStore:     es,
		reader:       r,
		cache: &statusCache{
			data: make(map[string]statusCacheEntry),
			ttl:  ttl,
		},
	}
}

func statusCacheTTL() time.Duration {
	n, err := strconv.Atoi(os.Getenv("STATUS_CACHE_TTL_SECONDS"))
	if err != nil || n <= 0 {
		return 60 * time.Second
	}
	return time.Duration(n) * time.Second
}

// GetStatus handles GET /api/v1/products/:productSlug/status.
// Returns a workload×environment tag matrix, cached for the configured TTL.
// When the cached entry is older than the TTL the response includes stale:true and
// the entry is evicted so the next request fetches fresh data.
// Concurrent cache misses for the same product are coalesced via singleflight to avoid
// hammering the gitops repo on thundering-herd scenarios.
func (h *StatusHandlers) GetStatus(c *gin.Context) {
	productSlug := c.Param("productSlug")

	if !checkProductAccess(c, productSlug) || !validateURLSlug(c, productSlug) {
		return
	}

	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
		return
	}

	if cached, hit := h.lookupCache(productSlug); hit {
		c.JSON(http.StatusOK, cached)
		return
	}

	// Coalesce concurrent cache misses: only one goroutine performs the fetch;
	// the rest wait and share the result. context.Background() is used here so the
	// cache-fill work survives the triggering request's cancellation — a client
	// disconnect should not abort a fetch that other waiters are sharing.
	// A 30s timeout prevents a hung git server from blocking indefinitely.
	fetchCtx, cancel := context.WithTimeout(context.Background(), statusFetchTimeout)
	defer cancel()
	v, err, _ := h.sf.Do(productSlug, func() (any, error) {
		return h.fetchAndCache(fetchCtx, product.ID, product.Slug)
	})
	if err != nil {
		var fetchErr *statusFetchError
		if errors.As(err, &fetchErr) {
			c.JSON(fetchErr.code, gin.H{"error": fetchErr.message})
			return
		}
		log.Printf("GetStatus product=%s: %v", productSlug, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	resp, ok := v.(StatusResponse)
	if !ok {
		log.Printf("GetStatus product=%s: unexpected singleflight result type %T", productSlug, v)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// fetchAndCache performs the full gitops read for a product, stores the result in the
// cache, and returns the StatusResponse. Returns *statusFetchError for predictable HTTP
// status codes (503, 404, etc.) and plain errors for unexpected failures.
func (h *StatusHandlers) fetchAndCache(ctx context.Context, productID, productSlug string) (StatusResponse, error) {
	envs, err := h.envStore.ListByProduct(ctx, productID)
	if err != nil {
		return StatusResponse{}, fmt.Errorf("list environments: %w", err)
	}

	workloadTags, err := h.collectTags(ctx, productSlug, envs)
	if err != nil {
		return StatusResponse{}, err
	}

	fillGaps(workloadTags, envs)

	fetchedAt := time.Now().UTC()
	resp := StatusResponse{
		Workloads: workloadTags,
		FetchedAt: fetchedAt.Format(time.RFC3339),
		Stale:     false,
	}

	h.cache.mu.Lock()
	h.cache.data[productSlug] = statusCacheEntry{result: resp, fetchedAt: fetchedAt}
	h.cache.mu.Unlock()

	return resp, nil
}

// lookupCache returns the cached StatusResponse for productSlug.
// On a fresh hit (within TTL) it returns (resp, true) with stale:false.
// On a stale hit it evicts the entry and returns (resp, true) with stale:true.
// On a miss it returns (zero, false).
func (h *StatusHandlers) lookupCache(productSlug string) (StatusResponse, bool) {
	h.cache.mu.Lock()
	defer h.cache.mu.Unlock()

	entry, hit := h.cache.data[productSlug]
	if !hit {
		return StatusResponse{}, false
	}
	if time.Since(entry.fetchedAt) < h.cache.ttl {
		return entry.result, true
	}
	// stale: evict so the next request re-fetches
	delete(h.cache.data, productSlug)
	staleResp := entry.result
	staleResp.Stale = true
	return staleResp, true
}

// collectTags calls ReadCurrentTags for each environment and aggregates results.
// ErrHelmReleaseNotFound and ErrHelmReleaseParseFailed skip the environment; workloads
// seen in other environments will show gitops.TagMissing for that column via fillGaps.
// Workloads exclusive to a skipped environment are omitted entirely.
// Returns *statusFetchError for service-level errors (503) and plain errors for infra failures.
func (h *StatusHandlers) collectTags(ctx context.Context, productSlug string, envs []domain.Environment) (map[string]map[string]string, error) {
	workloadTags := make(map[string]map[string]string)
	for _, env := range envs {
		tags, err := h.reader.ReadCurrentTags(ctx, productSlug, env.Slug)
		if err != nil {
			skip, fatal := classifyTagReadError(err, productSlug, env.Slug)
			if fatal != nil {
				return nil, fatal
			}
			if skip {
				continue
			}
		}
		for workload, tag := range tags {
			if workloadTags[workload] == nil {
				workloadTags[workload] = make(map[string]string)
			}
			workloadTags[workload][env.Slug] = tag
		}
	}
	return workloadTags, nil
}

// classifyTagReadError maps a ReadCurrentTags error to a skip decision or a fatal error.
// skip=true means the environment is silently excluded from the matrix.
// fatal!=nil means the entire status fetch must be aborted.
func classifyTagReadError(err error, productSlug, envSlug string) (skip bool, fatal error) {
	switch {
	case errors.Is(err, gitops.ErrHelmReleaseNotFound):
		log.Printf("collectTags product=%s env=%s: helmrelease not found, skipping", productSlug, envSlug)
		return true, nil
	case errors.Is(err, gitops.ErrHelmReleaseParseFailed):
		log.Printf("collectTags product=%s env=%s: helmrelease parse failed, skipping: %v", productSlug, envSlug, err)
		return true, nil
	case errors.Is(err, gitops.ErrGitOpsNotConfigured):
		return false, &statusFetchError{code: http.StatusServiceUnavailable, message: "deployment status is not available on this server"}
	default:
		return false, fmt.Errorf("env=%s: %w", envSlug, err)
	}
}

func fillGaps(workloadTags map[string]map[string]string, envs []domain.Environment) {
	for workload := range workloadTags {
		for _, env := range envs {
			if _, exists := workloadTags[workload][env.Slug]; !exists {
				workloadTags[workload][env.Slug] = gitops.TagMissing
			}
		}
	}
}
