package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// ErrComponentSlugConflict is returned when a component with the same (product_id, slug) already exists.
var ErrComponentSlugConflict = errors.New("component slug already exists for this product")

// ErrComponentNotFound is returned when the requested component does not exist.
var ErrComponentNotFound = errors.New("component not found")

// ErrComponentHasDeployments is returned when a component cannot be deleted because it has deployment records.
var ErrComponentHasDeployments = errors.New("component has active deployment records")

// ComponentStore is the persistence interface for components.
type ComponentStore interface {
	Create(ctx context.Context, c *domain.Component) error
	ListByProduct(ctx context.Context, productID string) ([]domain.Component, error)
	Delete(ctx context.Context, productID, slug string) error
}

type pgxComponentStore struct {
	pool *pgxpool.Pool
}

// NewComponentStore returns a ComponentStore backed by the given pgxpool.
func NewComponentStore(pool *pgxpool.Pool) ComponentStore {
	return &pgxComponentStore{pool: pool}
}

func (s *pgxComponentStore) Create(ctx context.Context, c *domain.Component) error {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO components (product_id, name, slug, gcr_image_path)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		c.ProductID, c.Name, c.Slug, c.GCRImagePath,
	)
	if err := row.Scan(&c.ID, &c.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return ErrComponentSlugConflict
		}
		return fmt.Errorf("create component: %w", err)
	}
	return nil
}

func (s *pgxComponentStore) ListByProduct(ctx context.Context, productID string) ([]domain.Component, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, product_id, name, slug, gcr_image_path, created_at
		 FROM components
		 WHERE product_id = $1
		 ORDER BY created_at ASC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	defer rows.Close()

	components := []domain.Component{}
	for rows.Next() {
		var c domain.Component
		if err := rows.Scan(&c.ID, &c.ProductID, &c.Name, &c.Slug, &c.GCRImagePath, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan component: %w", err)
		}
		components = append(components, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list components rows: %w", err)
	}
	return components, nil
}

func (s *pgxComponentStore) Delete(ctx context.Context, productID, slug string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM components WHERE product_id = $1 AND slug = $2`,
		productID, slug,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrComponentHasDeployments
		}
		return fmt.Errorf("delete component: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrComponentNotFound
	}
	return nil
}

