package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStatsTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip(skipDatabaseTestMessage)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedStatsFixture creates two products (one accessible to the test user, one not),
// environments, and deployments. Returns the slug of the accessible product.
func seedStatsFixture(t *testing.T, pool *pgxpool.Pool) (accessibleSlug string) {
	t.Helper()
	ctx := context.Background()

	// Product A — accessible
	var productAID string
	err := pool.QueryRow(ctx,
		`INSERT INTO products (name, slug, description) VALUES ('Stats Product A', 'stats-product-a', '')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&productAID)
	require.NoError(t, err)

	// Product B — not accessible to non-admin caller
	var productBID string
	err = pool.QueryRow(ctx,
		`INSERT INTO products (name, slug, description) VALUES ('Stats Product B', 'stats-product-b', '')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&productBID)
	require.NoError(t, err)

	// Two environments for Product A, one for Product B
	var envAID string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type, gitops_path)
		 VALUES ($1, 'prod', 'prod', 'production', 'envs/prod')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productAID,
	).Scan(&envAID)
	require.NoError(t, err)

	var envA2ID string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type, gitops_path)
		 VALUES ($1, 'dev', 'dev', 'dev', 'envs/dev')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productAID,
	).Scan(&envA2ID)
	require.NoError(t, err)

	var envBID string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type, gitops_path)
		 VALUES ($1, 'staging', 'staging', 'integration', 'envs/staging')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productBID,
	).Scan(&envBID)
	require.NoError(t, err)

	// Clean deployments for these products before inserting fixtures
	_, err = pool.Exec(ctx, `DELETE FROM deployments WHERE product_id IN ($1, $2)`, productAID, productBID)
	require.NoError(t, err)

	// Two recent deployments for Product A (within 24h)
	for _, comp := range []string{"api", "worker"} {
		_, err = pool.Exec(ctx,
			`INSERT INTO deployments
			 (product_id, environment_id, actor_display_name, component_name, environment_name, tag, deployed_at, outcome)
			 VALUES ($1, $2, 'Test Actor', $3, 'prod', 'v1.0.0', NOW() - INTERVAL '1 hour', 'success')`,
			productAID, envAID, comp,
		)
		require.NoError(t, err)
	}

	// One deployment for Product B (within 24h)
	_, err = pool.Exec(ctx,
		`INSERT INTO deployments
		 (product_id, environment_id, actor_display_name, component_name, environment_name, tag, deployed_at, outcome)
		 VALUES ($1, $2, 'Test Actor', 'svc', 'staging', 'v2.0.0', NOW() - INTERVAL '2 hours', 'success')`,
		productBID, envBID,
	)
	require.NoError(t, err)

	// One old deployment for Product A (older than 24h — must NOT appear in deployments_last_24h)
	_, err = pool.Exec(ctx,
		`INSERT INTO deployments
		 (product_id, environment_id, actor_display_name, component_name, environment_name, tag, deployed_at, outcome)
		 VALUES ($1, $2, 'Test Actor', 'api', 'prod', 'v0.9.0', $3, 'success')`,
		productAID, envAID, time.Now().UTC().Add(-25*time.Hour),
	)
	require.NoError(t, err)

	return "stats-product-a"
}

func TestStatsStore_GetStats_Admin(t *testing.T) {
	pool := newStatsTestPool(t)
	accessibleSlug := seedStatsFixture(t, pool)
	_ = accessibleSlug

	s := store.NewStatsStore(pool)
	stats, err := s.GetStats(context.Background(), nil, true)
	require.NoError(t, err)

	// Admin sees both products and all their environments
	assert.GreaterOrEqual(t, stats.ProductCount, 2, "admin should see at least both seeded products")
	assert.GreaterOrEqual(t, stats.EnvironmentCount, 3, "admin should see at least 3 environments")
	assert.GreaterOrEqual(t, stats.ComponentCount, 2, "admin should see at least 2 distinct components")
	assert.GreaterOrEqual(t, stats.Deployments24h, 3, "admin should count at least 3 recent deployments (rolling 24h)")
}

func TestStatsStore_GetStats_Scoped(t *testing.T) {
	pool := newStatsTestPool(t)
	accessibleSlug := seedStatsFixture(t, pool)

	s := store.NewStatsStore(pool)
	stats, err := s.GetStats(context.Background(), []string{accessibleSlug}, false)
	require.NoError(t, err)

	assert.Equal(t, 1, stats.ProductCount, "scoped: exactly 1 accessible product")
	assert.Equal(t, 2, stats.EnvironmentCount, "scoped: 2 environments for product A")
	assert.Equal(t, 2, stats.ComponentCount, "scoped: 2 distinct components (api, worker)")
	assert.Equal(t, 2, stats.Deployments24h, "scoped: 2 recent deployments within 24h for product A")
}

func TestStatsStore_GetStats_Empty(t *testing.T) {
	pool := newStatsTestPool(t)

	s := store.NewStatsStore(pool)
	stats, err := s.GetStats(context.Background(), []string{"slug-that-does-not-exist"}, false)
	require.NoError(t, err)

	assert.Equal(t, domain.Stats{}, stats, "unknown slug should return all-zero stats")
}

func TestStatsStore_GetStats_EmptySlugList_ReturnsZero(t *testing.T) {
	pool := newStatsTestPool(t)

	s := store.NewStatsStore(pool)
	// A non-admin caller with no accessible products (empty slug list) should get all-zero stats,
	// not an error. This is the "new user, no products yet" case.
	stats, err := s.GetStats(context.Background(), []string{}, false)
	require.NoError(t, err)

	assert.Equal(t, domain.Stats{}, stats, "empty slug list should return all-zero stats")
}

func TestStatsStore_GetStats_Deployments24hBoundary(t *testing.T) {
	pool := newStatsTestPool(t)
	ctx := context.Background()

	// Seed a dedicated product for boundary test
	var productID string
	err := pool.QueryRow(ctx,
		`INSERT INTO products (name, slug, description) VALUES ('Boundary Product', 'stats-boundary', '')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&productID)
	require.NoError(t, err)

	var envID string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type, gitops_path)
		 VALUES ($1, 'prod', 'bnd-prod', 'production', 'envs/bnd')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productID,
	).Scan(&envID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM deployments WHERE product_id = $1`, productID)
	require.NoError(t, err)

	// Deployment just inside 24h window
	_, err = pool.Exec(ctx,
		`INSERT INTO deployments
		 (product_id, environment_id, actor_display_name, component_name, environment_name, tag, deployed_at, outcome)
		 VALUES ($1, $2, 'Actor', 'api', 'prod', 'v1', $3, 'success')`,
		productID, envID, time.Now().UTC().Add(-23*time.Hour-59*time.Minute),
	)
	require.NoError(t, err)

	// Deployment just outside 24h window
	_, err = pool.Exec(ctx,
		`INSERT INTO deployments
		 (product_id, environment_id, actor_display_name, component_name, environment_name, tag, deployed_at, outcome)
		 VALUES ($1, $2, 'Actor', 'api', 'prod', 'v0', $3, 'success')`,
		productID, envID, time.Now().UTC().Add(-24*time.Hour-1*time.Minute),
	)
	require.NoError(t, err)

	s := store.NewStatsStore(pool)
	stats, err := s.GetStats(ctx, []string{"stats-boundary"}, false)
	require.NoError(t, err)

	assert.Equal(t, 1, stats.Deployments24h, "only the deployment inside 24h counts")
}
