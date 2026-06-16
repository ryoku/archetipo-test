package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeployIntegration_ConcurrentDeploymentRejection verifies the full lock lifecycle
// through the HTTP handler using a real PostgreSQL database.
//
// Scenario:
//  1. First request acquires the lock and begins a slow gitops write.
//  2. A concurrent second request for the same product-environment returns 409 with the
//     first caller's lock_holder and locked_since.
//  3. After the first request completes, a third request succeeds.
func TestDeployIntegration_ConcurrentDeploymentRejection(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	productID, envID, slug := insertDeployIntegrationFixtures(t, pool)

	lockStore := store.NewDeploymentLockStore(pool)

	product := &domain.Product{ID: productID, Name: "Integration Product", Slug: slug}
	env := &domain.Environment{
		ID:        envID,
		ProductID: productID,
		Name:      "dev",
		Slug:      "dev",
		Type:      "dev",
	}

	// Slow applier: blocks for 400 ms to give the second request time to collide.
	started := make(chan struct{})
	slowApplier := &mockGitOpsApplier{
		applyFn: func(_ context.Context, _ gitops.ApplyParams) (string, error) {
			close(started)
			time.Sleep(400 * time.Millisecond)
			return "sha-slow", nil
		},
	}
	// Fast applier for the third request.
	fastApplier := successApplier()

	identityEditor := &domain.UserIdentity{
		Sub:          "sara-1",
		Email:        "sara@example.com",
		Name:         "Sara DevOps",
		ProductRoles: map[string]domain.Role{slug: domain.RoleEditor},
	}

	ps := &mockProductStore{
		getBySlugFn: func(_ context.Context, s string) (*domain.Product, error) {
			if s == product.Slug {
				return product, nil
			}
			return nil, store.ErrNotFound
		},
	}
	es := &mockEnvironmentStore{
		getByIDFn: func(_ context.Context, pID, eID string) (*domain.Environment, error) {
			if pID == productID && eID == envID {
				return env, nil
			}
			return nil, store.ErrEnvironmentNotFound
		},
	}
	makeRouter := func(applier handlers.GitOpsApplier) *gin.Engine {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := handlers.NewDeploymentHandlers(ps, es, lockStore, noopDeploymentStore(), applier, "")
		injectIdentity := func(c *gin.Context) {
			c.Set(domain.IdentityContextKey, identityEditor)
			c.Next()
		}
		api := r.Group("/api/v1", injectIdentity)
		api.POST("/products/:productSlug/environments/:environmentID/deployments",
			middleware.RequireRole(domain.RoleEditor), h.Deploy)
		return r
	}

	doRequest := func(r *gin.Engine) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{
			"workload": "integ-svc",
			"tag":      "v1.0.0",
		})
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/products/"+slug+"/environments/"+envID+"/deployments",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	r1 := makeRouter(slowApplier)
	r2 := makeRouter(slowApplier)
	r3 := makeRouter(fastApplier)

	var (
		wg sync.WaitGroup
		w1 *httptest.ResponseRecorder
		w2 *httptest.ResponseRecorder
	)

	// First request: acquires lock and holds it for 400 ms.
	wg.Add(1)
	go func() {
		defer wg.Done()
		w1 = doRequest(r1)
	}()

	// Wait until the slow applier has started (lock is definitely held).
	<-started

	// Second request: should be rejected with 409.
	w2 = doRequest(r2)

	wg.Wait()

	// First request must succeed.
	assert.Equal(t, http.StatusAccepted, w1.Code, "first request should succeed")

	// Second request must return 409 with correct lock holder.
	assert.Equal(t, http.StatusConflict, w2.Code, "second concurrent request should return 409")
	var conflictResp map[string]string
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &conflictResp))
	assert.Equal(t, "deployment in progress", conflictResp["error"])
	assert.Equal(t, "Sara DevOps", conflictResp["lock_holder"])
	assert.NotEmpty(t, conflictResp["locked_since"])

	// Third request after the lock is released must succeed.
	w3 := doRequest(r3)
	assert.Equal(t, http.StatusAccepted, w3.Code, "third request after lock release should succeed")
}

// TestDeployIntegration_FullLifecycle verifies the complete deployment lifecycle:
// POST → 202 with deployment_id → GET on that deployment ID → commit_sha and outcome=success present.
func TestDeployIntegration_FullLifecycle(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	productID, envID, slug := insertDeployIntegrationFixtures(t, pool)

	lockStore := store.NewDeploymentLockStore(pool)
	deploymentStore := store.NewDeploymentStore(pool)

	product := &domain.Product{ID: productID, Name: "Lifecycle Product", Slug: slug}
	env := &domain.Environment{
		ID: envID, ProductID: productID, Name: "dev", Slug: "dev", Type: "dev",
	}

	const testCommitSHA = "cafebabe1234567890abcdef"

	applier := &mockGitOpsApplier{
		applyFn: func(_ context.Context, _ gitops.ApplyParams) (string, error) {
			return testCommitSHA, nil
		},
	}

	identity := &domain.UserIdentity{
		Sub:          "lifecycle-user-sub",
		Email:        "lifecycle@example.com",
		Name:         "Lifecycle User",
		ProductRoles: map[string]domain.Role{slug: domain.RoleEditor},
	}

	ps := &mockProductStore{
		getBySlugFn: func(_ context.Context, s string) (*domain.Product, error) {
			if s == product.Slug {
				return product, nil
			}
			return nil, store.ErrNotFound
		},
		getByIDFn: func(_ context.Context, id string) (*domain.Product, error) {
			if id == product.ID {
				return product, nil
			}
			return nil, store.ErrNotFound
		},
	}
	es := &mockEnvironmentStore{
		getByIDFn: func(_ context.Context, pID, eID string) (*domain.Environment, error) {
			if pID == productID && eID == envID {
				return env, nil
			}
			return nil, store.ErrEnvironmentNotFound
		},
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(ps, es, lockStore, deploymentStore, applier, "")
	injectIdentity := func(c *gin.Context) {
		c.Set(domain.IdentityContextKey, identity)
		c.Next()
	}
	api := r.Group("/api/v1", injectIdentity)
	api.POST("/products/:productSlug/environments/:environmentID/deployments",
		middleware.RequireRole(domain.RoleEditor), h.Deploy)
	api.GET("/deployments/:deploymentID", h.GetDeployment)

	// Step 1: POST → expect 202 with deployment_id.
	postBody, _ := json.Marshal(map[string]string{"workload": "api", "tag": "v2.0.0"})
	postReq := httptest.NewRequest(http.MethodPost,
		"/api/v1/products/"+slug+"/environments/"+envID+"/deployments",
		bytes.NewReader(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	require.Equal(t, http.StatusAccepted, postW.Code, "POST should return 202")
	var postResp map[string]string
	require.NoError(t, json.Unmarshal(postW.Body.Bytes(), &postResp))
	deploymentID := postResp["deployment_id"]
	require.NotEmpty(t, deploymentID, "deployment_id must be present in 202 response")

	// Step 2: GET → expect 200 with commit_sha and outcome=success.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/"+deploymentID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	require.Equal(t, http.StatusOK, getW.Code, "GET should return 200")
	var getResp map[string]string
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &getResp))
	assert.Equal(t, testCommitSHA, getResp["commit_sha"])
	assert.Equal(t, domain.OutcomeSuccess, getResp["outcome"])
	assert.Equal(t, "v2.0.0", getResp["tag"])
	assert.Equal(t, "api", getResp["workload"])
}

// insertDeployIntegrationFixtures inserts a product and environment for the integration test.
func insertDeployIntegrationFixtures(t *testing.T, pool *pgxpool.Pool) (productID, envID, slug string) {
	t.Helper()
	ctx := context.Background()
	slug = integTestSlug(t)

	err := pool.QueryRow(ctx,
		`INSERT INTO products (name, slug, description) VALUES ($1, $2, $3) RETURNING id`,
		"Integration Product "+slug, slug, "",
	).Scan(&productID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type) VALUES ($1, $2, $3, $4) RETURNING id`,
		productID, "dev", "dev", "dev",
	).Scan(&envID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM deployments WHERE product_id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM deployment_locks WHERE product_id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM environments WHERE product_id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID)
	})
	return productID, envID, slug
}

// integTestSlug returns a unique, URL-safe slug derived from the test name.
func integTestSlug(t *testing.T) string {
	t.Helper()
	s := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '-'
		}
	}, t.Name())
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "test"
	}
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	return s
}
