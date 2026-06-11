package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// ErrEnvironmentNameConflict is returned when an environment with the same name already exists for this product.
var ErrEnvironmentNameConflict = errors.New("environment name already exists for this product")

// ErrEnvironmentSlugConflict is returned when an environment with the same slug already exists for this product.
var ErrEnvironmentSlugConflict = errors.New("environment slug already exists for this product")

// ErrEnvironmentNotFound is returned when the requested environment does not exist.
var ErrEnvironmentNotFound = errors.New("environment not found")

// ErrEnvironmentHasDeployments is returned when an environment cannot be deleted because it has deployment records.
var ErrEnvironmentHasDeployments = errors.New("environment has active deployment records")

// EnvironmentStore is the persistence interface for environments.
type EnvironmentStore interface {
	Create(ctx context.Context, e *domain.Environment) error
	ListByProduct(ctx context.Context, productID string) ([]domain.Environment, error)
	GetByID(ctx context.Context, productID, environmentID string) (*domain.Environment, error)
	Delete(ctx context.Context, productID, environmentID string) error
}

type pgxEnvironmentStore struct {
	pool *pgxpool.Pool
}

// NewEnvironmentStore returns an EnvironmentStore backed by the given pgxpool.
func NewEnvironmentStore(pool *pgxpool.Pool) EnvironmentStore {
	return &pgxEnvironmentStore{pool: pool}
}

func (s *pgxEnvironmentStore) Create(ctx context.Context, e *domain.Environment) error {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type, overlay_path)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		e.ProductID, e.Name, e.Slug, e.Type, e.OverlayPath,
	)
	if err := row.Scan(&e.ID, &e.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "slug") {
				return ErrEnvironmentSlugConflict
			}
			return ErrEnvironmentNameConflict
		}
		return fmt.Errorf("create environment: %w", err)
	}
	return nil
}

func (s *pgxEnvironmentStore) ListByProduct(ctx context.Context, productID string) ([]domain.Environment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, product_id, name, slug, type, overlay_path, created_at
		 FROM environments
		 WHERE product_id = $1
		 ORDER BY created_at ASC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	defer rows.Close()

	environments := []domain.Environment{}
	for rows.Next() {
		var e domain.Environment
		if err := rows.Scan(&e.ID, &e.ProductID, &e.Name, &e.Slug, &e.Type, &e.OverlayPath, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan environment: %w", err)
		}
		environments = append(environments, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list environments rows: %w", err)
	}
	return environments, nil
}

func (s *pgxEnvironmentStore) GetByID(ctx context.Context, productID, environmentID string) (*domain.Environment, error) {
	var e domain.Environment
	err := s.pool.QueryRow(ctx,
		`SELECT id, product_id, name, slug, type, overlay_path, created_at
		 FROM environments
		 WHERE product_id = $1 AND id = $2`,
		productID, environmentID,
	).Scan(&e.ID, &e.ProductID, &e.Name, &e.Slug, &e.Type, &e.OverlayPath, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("get environment by id: %w", err)
	}
	return &e, nil
}

func (s *pgxEnvironmentStore) Delete(ctx context.Context, productID, environmentID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM environments WHERE product_id = $1 AND id = $2`,
		productID, environmentID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrEnvironmentHasDeployments
		}
		return fmt.Errorf("delete environment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEnvironmentNotFound
	}
	return nil
}
