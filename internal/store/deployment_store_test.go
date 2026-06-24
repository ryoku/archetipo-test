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
		`INSERT INTO environments (product_id, name, slug, type, gitops_path)
		 VALUES ($1, 'staging', 'staging', 'integration', 'envs/staging')
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
