package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// ErrDeploymentNotFound is returned when a deployment with the given ID does not exist.
var ErrDeploymentNotFound = errors.New("deployment not found")

// DeploymentStore persists and retrieves deployment records.
type DeploymentStore interface {
	// Create inserts a new deployment record and populates d.ID and d.DeployedAt.
	Create(ctx context.Context, d *domain.Deployment) error
	// GetByID returns the deployment with the given ID, or ErrDeploymentNotFound.
	GetByID(ctx context.Context, id string) (*domain.Deployment, error)
}

type pgxDeploymentStore struct {
	pool *pgxpool.Pool
}

// NewDeploymentStore returns a DeploymentStore backed by the given pgxpool.
func NewDeploymentStore(pool *pgxpool.Pool) DeploymentStore {
	return &pgxDeploymentStore{pool: pool}
}

func (s *pgxDeploymentStore) Create(ctx context.Context, d *domain.Deployment) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO deployments
		    (actor_sub, product_id, environment_id, workload, tag, commit_sha, outcome, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, deployed_at`,
		d.ActorSub, d.ProductID, d.EnvironmentID, d.Workload, d.Tag,
		d.CommitSHA, d.Outcome, d.ErrorMessage,
	).Scan(&d.ID, &d.DeployedAt)
	if err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}
	return nil
}

func (s *pgxDeploymentStore) GetByID(ctx context.Context, id string) (*domain.Deployment, error) {
	var d domain.Deployment
	err := s.pool.QueryRow(ctx,
		`SELECT id, actor_sub, product_id, environment_id, workload, tag,
		        deployed_at, commit_sha, outcome, error_message
		 FROM deployments WHERE id = $1`,
		id,
	).Scan(
		&d.ID, &d.ActorSub, &d.ProductID, &d.EnvironmentID,
		&d.Workload, &d.Tag, &d.DeployedAt, &d.CommitSHA,
		&d.Outcome, &d.ErrorMessage,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isInvalidUUIDSyntax(err) {
			return nil, ErrDeploymentNotFound
		}
		return nil, fmt.Errorf("get deployment: %w", err)
	}
	return &d, nil
}

// isInvalidUUIDSyntax reports whether err is a PostgreSQL invalid text representation
// error for a UUID column (SQLSTATE 22P02). This occurs when a non-UUID string is passed
// as a UUID parameter, which should be treated as not-found rather than a server error.
func isInvalidUUIDSyntax(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
