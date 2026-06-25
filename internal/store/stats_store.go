package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// StatsStore aggregates platform-level metrics scoped to accessible products.
type StatsStore interface {
	// GetStats returns aggregate counts for the products identified by slugs.
	// When isAdmin is true all products are included and slugs is ignored.
	GetStats(ctx context.Context, slugs []string, isAdmin bool) (domain.Stats, error)
}

type pgxStatsStore struct {
	pool *pgxpool.Pool
}

// NewStatsStore returns a StatsStore backed by the given pgxpool.
func NewStatsStore(pool *pgxpool.Pool) StatsStore {
	return &pgxStatsStore{pool: pool}
}

// GetStats runs a single CTE query that aggregates product, environment, component,
// and deployment counts scoped to the caller's accessible products.
// component_count uses COUNT(DISTINCT component_name) FROM deployments because the
// components table was dropped in migration 007; deployment history is the only
// persisted proxy for component identity.
func (s *pgxStatsStore) GetStats(ctx context.Context, slugs []string, isAdmin bool) (domain.Stats, error) {
	const queryAdmin = `
		WITH accessible AS (
			SELECT id FROM products WHERE archived_at IS NULL
		)
		SELECT
			(SELECT COUNT(*) FROM accessible)                                              AS product_count,
			(SELECT COUNT(*) FROM environments WHERE product_id IN (SELECT id FROM accessible)) AS environment_count,
			(SELECT COUNT(DISTINCT component_name)
			 FROM deployments WHERE product_id IN (SELECT id FROM accessible))             AS component_count,
			(SELECT COUNT(*)
			 FROM deployments
			 WHERE product_id IN (SELECT id FROM accessible)
			   AND deployed_at >= NOW() - INTERVAL '24 hours')                             AS deployments_today
	`

	const queryScoped = `
		WITH accessible AS (
			SELECT id FROM products WHERE slug = ANY($1) AND archived_at IS NULL
		)
		SELECT
			(SELECT COUNT(*) FROM accessible)                                              AS product_count,
			(SELECT COUNT(*) FROM environments WHERE product_id IN (SELECT id FROM accessible)) AS environment_count,
			(SELECT COUNT(DISTINCT component_name)
			 FROM deployments WHERE product_id IN (SELECT id FROM accessible))             AS component_count,
			(SELECT COUNT(*)
			 FROM deployments
			 WHERE product_id IN (SELECT id FROM accessible)
			   AND deployed_at >= NOW() - INTERVAL '24 hours')                             AS deployments_today
	`

	var stats domain.Stats
	var err error
	if isAdmin {
		err = s.pool.QueryRow(ctx, queryAdmin).Scan(
			&stats.ProductCount,
			&stats.EnvironmentCount,
			&stats.ComponentCount,
			&stats.DeploymentsToday,
		)
	} else {
		err = s.pool.QueryRow(ctx, queryScoped, slugs).Scan(
			&stats.ProductCount,
			&stats.EnvironmentCount,
			&stats.ComponentCount,
			&stats.DeploymentsToday,
		)
	}
	if err != nil {
		return domain.Stats{}, fmt.Errorf("get stats: %w", err)
	}
	return stats, nil
}
