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

func strPtr(s string) *string { return &s }

func newDeploymentTestPool(t *testing.T) *pgxpool.Pool {
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

func seedProductAndEnv(t *testing.T, pool *pgxpool.Pool) (productID, envID string) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`INSERT INTO products (name, slug, description) VALUES ('Test Product', 'test-product-deploy', 'desc')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&productID)
	require.NoError(t, err)

	err = pool.QueryRow(context.Background(),
		`INSERT INTO environments (product_id, name, slug, type)
		 VALUES ($1, 'staging', 'staging', 'integration')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productID,
	).Scan(&envID)
	require.NoError(t, err)
	return productID, envID
}

func cleanDeployments(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "DELETE FROM deployments")
	require.NoError(t, err)
}

func TestDeploymentStore_Create(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)

	s := store.NewDeploymentStore(pool)
	d := &domain.Deployment{
		ProductID:        productID,
		EnvironmentID:    envID,
		ActorDisplayName: "Marco Andreoli",
		ComponentName:    "api",
		EnvironmentName:  "staging",
		Tag:              "v1.2.3",
		DeployedAt:       time.Now().UTC().Truncate(time.Microsecond),
		CommitSHA:        strPtr("abc123"),
		Outcome:          "success",
	}

	err := s.Create(context.Background(), d)
	require.NoError(t, err)
	assert.NotEmpty(t, d.ID)
}

func seedNDeployments(t *testing.T, s store.DeploymentStore, productID, envID string, n int) {
	t.Helper()
	base := time.Now().UTC().Truncate(time.Microsecond)
	for i := range n {
		err := s.Create(context.Background(), &domain.Deployment{
			ProductID:        productID,
			EnvironmentID:    envID,
			ActorDisplayName: "Actor",
			ComponentName:    "api",
			EnvironmentName:  "staging",
			Tag:              "v1.0." + string(rune('0'+i)),
			DeployedAt:       base.Add(time.Duration(i) * time.Minute),
			CommitSHA:        strPtr("sha" + string(rune('0'+i))),
			Outcome:          "success",
		})
		require.NoError(t, err)
	}
}

func TestDeploymentStore_GetByID(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)

	s := store.NewDeploymentStore(pool)
	d := &domain.Deployment{
		ProductID:        productID,
		EnvironmentID:    envID,
		ActorDisplayName: "Sara DevOps",
		ComponentName:    "api",
		EnvironmentName:  "staging",
		Tag:              "v1.2.3",
		CommitSHA:        strPtr("abc123def456"),
		Outcome:          domain.OutcomeSuccess,
	}

	require.NoError(t, s.Create(context.Background(), d))
	assert.NotEmpty(t, d.ID)
	assert.False(t, d.DeployedAt.IsZero())

	got, err := s.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, d.ID, got.ID)
	assert.Equal(t, d.ActorDisplayName, got.ActorDisplayName)
	assert.Equal(t, d.ComponentName, got.ComponentName)
	assert.Equal(t, d.EnvironmentName, got.EnvironmentName)
	assert.Equal(t, d.ProductID, got.ProductID)
	assert.Equal(t, d.EnvironmentID, got.EnvironmentID)
	assert.Equal(t, d.Tag, got.Tag)
	require.NotNil(t, got.CommitSHA)
	assert.Equal(t, *d.CommitSHA, *got.CommitSHA)
	assert.Equal(t, d.Outcome, got.Outcome)
	assert.Nil(t, got.ErrorMessage)
}

func TestDeploymentStore_GetByID_NotFound(t *testing.T) {
	pool := newDeploymentTestPool(t)
	s := store.NewDeploymentStore(pool)

	_, err := s.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)
}

func TestDeploymentStore_GetByID_MalformedUUID_Returns404Sentinel(t *testing.T) {
	pool := newDeploymentTestPool(t)
	s := store.NewDeploymentStore(pool)

	_, err := s.GetByID(context.Background(), "not-a-uuid")
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)
}

func TestDeploymentStore_ListByProduct(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)
	s := store.NewDeploymentStore(pool)
	seedNDeployments(t, s, productID, envID, 5)

	deployments, total, err := s.ListByProduct(context.Background(), productID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, deployments, 5)
	assert.True(t, deployments[0].DeployedAt.After(deployments[1].DeployedAt))
}

func TestDeploymentStore_ListByProduct_Pagination(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)
	s := store.NewDeploymentStore(pool)
	seedNDeployments(t, s, productID, envID, 5)

	page1, total, err := s.ListByProduct(context.Background(), productID, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 3)

	page2, total2, err := s.ListByProduct(context.Background(), productID, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, page2, 2)
}

func TestDeploymentStore_ListAll(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)
	s := store.NewDeploymentStore(pool)
	seedNDeployments(t, s, productID, envID, 3)

	deployments, total, err := s.ListAll(context.Background(), 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, deployments, 3)
}

func TestDeploymentStore_Create_FailureOutcome(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)

	s := store.NewDeploymentStore(pool)
	errMsg := "gitops push failed"
	d := &domain.Deployment{
		ProductID:        productID,
		EnvironmentID:    envID,
		ActorDisplayName: "Marco Andreoli",
		ComponentName:    "api",
		EnvironmentName:  "staging",
		Tag:              "v1.2.3",
		DeployedAt:       time.Now().UTC().Truncate(time.Microsecond),
		CommitSHA:        nil,
		Outcome:          "failure",
		ErrorMessage:     &errMsg,
	}

	err := s.Create(context.Background(), d)
	require.NoError(t, err)

	deployments, total, err := s.ListByProduct(context.Background(), productID, 1, 20)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, "failure", deployments[0].Outcome)
	assert.Nil(t, deployments[0].CommitSHA)
	require.NotNil(t, deployments[0].ErrorMessage)
	assert.Equal(t, errMsg, *deployments[0].ErrorMessage)
}

func TestDeploymentStore_UpdateOutcome(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)

	s := store.NewDeploymentStore(pool)
	d := &domain.Deployment{
		ProductID:        productID,
		EnvironmentID:    envID,
		ActorDisplayName: "Luigi Infra",
		ComponentName:    "worker",
		EnvironmentName:  "staging",
		Tag:              "v2.0.0",
		CommitSHA:        nil,
		Outcome:          domain.OutcomeInProgress,
	}
	require.NoError(t, s.Create(context.Background(), d))

	sha := "abc123"
	err := s.UpdateOutcome(context.Background(), d.ID, domain.OutcomeSuccess, &sha, nil)
	require.NoError(t, err)

	got, err := s.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeSuccess, got.Outcome)
	require.NotNil(t, got.CommitSHA)
	assert.Equal(t, "abc123", *got.CommitSHA)
	assert.Nil(t, got.ErrorMessage)

	// Unknown ID must return ErrDeploymentNotFound.
	err = s.UpdateOutcome(context.Background(), "00000000-0000-0000-0000-000000000000", domain.OutcomeSuccess, &sha, nil)
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)
}

func TestDeploymentStore_Delete(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)

	s := store.NewDeploymentStore(pool)
	d := &domain.Deployment{
		ProductID:        productID,
		EnvironmentID:    envID,
		ActorDisplayName: "Sara DevOps",
		ComponentName:    "api",
		EnvironmentName:  "staging",
		Tag:              "v1.0.0",
		CommitSHA:        nil,
		Outcome:          domain.OutcomeInProgress,
	}
	require.NoError(t, s.Create(context.Background(), d))

	require.NoError(t, s.Delete(context.Background(), d.ID))

	_, err := s.GetByID(context.Background(), d.ID)
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)

	// Deleting a non-existent record returns ErrDeploymentNotFound.
	err = s.Delete(context.Background(), "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)
}

func TestDeploymentStore_ListActivity(t *testing.T) {
	pool := newDeploymentTestPool(t)
	cleanDeployments(t, pool)

	// Seed two distinct products with their own environments.
	var productID1, envID1, productID2, envID2 string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO products (name, slug, description)
		 VALUES ('Activity Product 1', 'activity-product-1', 'desc')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&productID1))
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO environments (product_id, name, slug, type)
		 VALUES ($1, 'staging', 'staging-act1', 'integration')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productID1,
	).Scan(&envID1))

	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO products (name, slug, description)
		 VALUES ('Activity Product 2', 'activity-product-2', 'desc')
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&productID2))
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO environments (product_id, name, slug, type)
		 VALUES ($1, 'staging', 'staging-act2', 'integration')
		 ON CONFLICT (product_id, slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		productID2,
	).Scan(&envID2))

	// Insert 3 deployments with explicit deployed_at values so ordering is deterministic.
	base := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO deployments
		 (product_id, environment_id, actor_display_name, component_name, environment_name,
		  tag, deployed_at, commit_sha, outcome)
		 VALUES
		   ($1, $2, 'Actor', 'api', 'staging', 'v1.0.0', $5, NULL, 'success'),
		   ($3, $4, 'Actor', 'api', 'staging', 'v1.0.1', $6, NULL, 'success'),
		   ($1, $2, 'Actor', 'api', 'staging', 'v1.0.2', $7, NULL, 'success')`,
		productID1, envID1, productID2, envID2,
		base.Add(-2*time.Minute), base.Add(-1*time.Minute), base,
	)
	require.NoError(t, err)

	s := store.NewDeploymentStore(pool)
	activity, err := s.ListActivity(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, activity, 2)

	// Most recent first.
	assert.True(t, activity[0].DeployedAt.After(activity[1].DeployedAt))

	// ProductSlug must be populated for each returned deployment.
	assert.Equal(t, "activity-product-1", activity[0].ProductSlug)
	assert.Equal(t, "activity-product-2", activity[1].ProductSlug)
}

func TestDeploymentStore_MarkStaleInProgress(t *testing.T) {
	pool := newDeploymentTestPool(t)
	productID, envID := seedProductAndEnv(t, pool)
	cleanDeployments(t, pool)

	s := store.NewDeploymentStore(pool)

	// Insert a stale in_progress deployment with deployed_at 10 minutes in the past.
	var staleID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO deployments
		 (product_id, environment_id, actor_display_name, component_name, environment_name,
		  tag, deployed_at, commit_sha, outcome)
		 VALUES ($1, $2, 'Actor', 'api', 'staging', 'v3.0.0', NOW() - INTERVAL '10 minutes', NULL, 'in_progress')
		 RETURNING id`,
		productID, envID,
	).Scan(&staleID))

	// Insert a recent in_progress deployment that must NOT be touched.
	recent := &domain.Deployment{
		ProductID:        productID,
		EnvironmentID:    envID,
		ActorDisplayName: "Actor",
		ComponentName:    "api",
		EnvironmentName:  "staging",
		Tag:              "v3.0.1",
		CommitSHA:        nil,
		Outcome:          domain.OutcomeInProgress,
	}
	require.NoError(t, s.Create(context.Background(), recent))

	require.NoError(t, s.MarkStaleInProgress(context.Background(), 5*time.Minute))

	// Stale deployment must now be failure with error_message = "timeout".
	stale, err := s.GetByID(context.Background(), staleID)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeFailure, stale.Outcome)
	require.NotNil(t, stale.ErrorMessage)
	assert.Equal(t, "timeout", *stale.ErrorMessage)

	// Recent deployment must remain in_progress.
	got, err := s.GetByID(context.Background(), recent.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeInProgress, got.Outcome)
	assert.Nil(t, got.ErrorMessage)
}
