package handlers

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

const statsCacheTTL = 30 * time.Second

// adminCacheKey is the stable cache key used when the caller is a DevOps Admin.
const adminCacheKey = "__admin__"

type statsCacheEntry struct {
	result    domain.Stats
	fetchedAt time.Time
}

type statsCache struct {
	mu   sync.Mutex
	data map[string]statsCacheEntry
	ttl  time.Duration
}

func (c *statsCache) get(key string) (domain.Stats, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.data[key]
	if !ok {
		return domain.Stats{}, false
	}
	if time.Since(entry.fetchedAt) >= c.ttl {
		delete(c.data, key)
		return domain.Stats{}, false
	}
	return entry.result, true
}

func (c *statsCache) set(key string, s domain.Stats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = statsCacheEntry{result: s, fetchedAt: time.Now()}
}

// StatsHandlers bundles HTTP handlers for the /stats endpoint.
type StatsHandlers struct {
	store store.StatsStore
	cache *statsCache
}

// NewStatsHandlers returns a StatsHandlers wired to the given store with a 30s TTL.
func NewStatsHandlers(s store.StatsStore) *StatsHandlers {
	return NewStatsHandlersWithTTL(s, statsCacheTTL)
}

// NewStatsHandlersWithTTL is identical to NewStatsHandlers but accepts an explicit TTL.
func NewStatsHandlersWithTTL(s store.StatsStore, ttl time.Duration) *StatsHandlers {
	return &StatsHandlers{
		store: s,
		cache: &statsCache{
			data: make(map[string]statsCacheEntry),
			ttl:  ttl,
		},
	}
}

// GetStats handles GET /api/v1/stats.
// Returns aggregate counts scoped to the caller's accessible products.
// Results are cached server-side for 30 seconds.
func (h *StatsHandlers) GetStats(c *gin.Context) {
	identity, ok := middleware.IdentityFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	slugs := accessibleSlugs(identity)
	cacheKey := cacheKeyFromSlugs(identity.IsDevOpsAdmin, slugs)

	if cached, hit := h.cache.get(cacheKey); hit {
		c.JSON(http.StatusOK, cached)
		return
	}

	stats, err := h.store.GetStats(c.Request.Context(), slugs, identity.IsDevOpsAdmin)
	if err != nil {
		log.Printf("GetStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	h.cache.set(cacheKey, stats)
	c.JSON(http.StatusOK, stats)
}

// cacheKeyFromSlugs returns a deterministic cache key given a pre-computed slug list.
func cacheKeyFromSlugs(isAdmin bool, slugs []string) string {
	if isAdmin {
		return adminCacheKey
	}
	sort.Strings(slugs)
	return strings.Join(slugs, ",")
}

// accessibleSlugs returns the product slugs the caller has a role on.
func accessibleSlugs(identity *domain.UserIdentity) []string {
	slugs := make([]string, 0, len(identity.ProductRoles))
	for slug := range identity.ProductRoles {
		slugs = append(slugs, slug)
	}
	return slugs
}
