package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// ErrDeploymentNotFound is returned when a deployment with the given ID does not exist.
var ErrDeploymentNotFound = errors.New("deployment not found")

// DeploymentStore persists and retrieves deployment records.
type DeploymentStore interface {
	// Create inserts a new deployment record and populates d.ID.
	Create(ctx context.Context, d *domain.Deployment) error
	// GetByID returns the deployment with the given ID, or ErrDeploymentNotFound.
	GetByID(ctx context.Context, id string) (*domain.Deployment, error)
	// ListByProduct returns deployment records for a product, ordered by deployed_at DESC.
	// Returns records for the given page (1-based) and the total count.
	ListByProduct(ctx context.Context, productID string, page, pageSize int) ([]domain.Deployment, int, error)
	// ListAll returns all deployment records across all products, ordered by deployed_at DESC.
	ListAll(ctx context.Context, page, pageSize int) ([]domain.Deployment, int, error)
	// UpdateOutcome updates outcome, commit_sha and error_message for an existing deployment.
	UpdateOutcome(ctx context.Context, id string, outcome domain.DeploymentOutcome, commitSHA *string, errorMessage *string) error
	// Delete removes the deployment record with the given ID, or returns ErrDeploymentNotFound.
	Delete(ctx context.Context, id string) error
	// ListActivity returns the N most recent deployments across all products, ordered by deployed_at DESC.
	// ProductSlug is populated via JOIN with products.
	ListActivity(ctx context.Context, limit int) ([]domain.Deployment, error)
	// MarkStaleInProgress marks in_progress deployments older than olderThan (measured from deployed_at) as failure with error_message = "deployment status never finalized".
	MarkStaleInProgress(ctx context.Context, olderThan time.Duration) error
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
		 (product_id, environment_id, actor_display_name, component_name, environment_name,
		  tag, deployed_at, commit_sha, outcome, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7, $8, $9)
		 RETURNING id, deployed_at`,
		d.ProductID, d.EnvironmentID, d.ActorDisplayName, d.ComponentName, d.EnvironmentName,
		d.Tag, d.CommitSHA, d.Outcome, d.ErrorMessage,
	).Scan(&d.ID, &d.DeployedAt)
	if err != nil {
		return fmt.Errorf("deployment store create: %w", err)
	}
	return nil
}

func (s *pgxDeploymentStore) GetByID(ctx context.Context, id string) (*domain.Deployment, error) {
	var d domain.Deployment
	err := s.pool.QueryRow(ctx,
		`SELECT id, product_id, environment_id, actor_display_name, component_name,
		        environment_name, tag, deployed_at, commit_sha, outcome, error_message
		 FROM deployments WHERE id = $1`,
		id,
	).Scan(
		&d.ID, &d.ProductID, &d.EnvironmentID, &d.ActorDisplayName, &d.ComponentName,
		&d.EnvironmentName, &d.Tag, &d.DeployedAt, &d.CommitSHA, &d.Outcome, &d.ErrorMessage,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isInvalidUUIDSyntax(err) {
			return nil, ErrDeploymentNotFound
		}
		return nil, fmt.Errorf("get deployment: %w", err)
	}
	return &d, nil
}

func (s *pgxDeploymentStore) ListByProduct(ctx context.Context, productID string, page, pageSize int) ([]domain.Deployment, int, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM deployments WHERE product_id = $1`, productID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("deployment store count by product: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, product_id, environment_id, actor_display_name, component_name,
		        environment_name, tag, deployed_at, commit_sha, outcome, error_message
		 FROM deployments
		 WHERE product_id = $1
		 ORDER BY deployed_at DESC
		 LIMIT $2 OFFSET $3`,
		productID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("deployment store list by product: %w", err)
	}
	defer rows.Close()

	deployments, err := scanDeployments(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("deployment store list by product: %w", err)
	}
	return deployments, total, nil
}

func (s *pgxDeploymentStore) ListAll(ctx context.Context, page, pageSize int) ([]domain.Deployment, int, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM deployments`,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("deployment store count all: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, product_id, environment_id, actor_display_name, component_name,
		        environment_name, tag, deployed_at, commit_sha, outcome, error_message
		 FROM deployments
		 ORDER BY deployed_at DESC
		 LIMIT $1 OFFSET $2`,
		pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("deployment store list all: %w", err)
	}
	defer rows.Close()

	deployments, err := scanDeployments(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("deployment store list all: %w", err)
	}
	return deployments, total, nil
}

func (s *pgxDeploymentStore) UpdateOutcome(ctx context.Context, id string, outcome domain.DeploymentOutcome, commitSHA *string, errorMessage *string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE deployments SET outcome = $2, commit_sha = $3, error_message = $4 WHERE id = $1`,
		id, outcome, commitSHA, errorMessage,
	)
	if err != nil {
		return fmt.Errorf("update deployment outcome: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeploymentNotFound
	}
	return nil
}

func (s *pgxDeploymentStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM deployments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete deployment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeploymentNotFound
	}
	return nil
}

func (s *pgxDeploymentStore) ListActivity(ctx context.Context, limit int) ([]domain.Deployment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.product_id, p.slug AS product_slug, d.environment_id,
		        d.actor_display_name, d.component_name, d.environment_name,
		        d.tag, d.deployed_at, d.commit_sha, d.outcome, d.error_message
		 FROM deployments d
		 LEFT JOIN products p ON p.id = d.product_id
		 ORDER BY d.deployed_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("deployment store list activity: %w", err)
	}
	defer rows.Close()

	deployments, err := scanActivityDeployments(rows)
	if err != nil {
		return nil, fmt.Errorf("deployment store list activity: %w", err)
	}
	return deployments, nil
}

func (s *pgxDeploymentStore) MarkStaleInProgress(ctx context.Context, olderThan time.Duration) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE deployments
		 SET outcome = 'failure', error_message = 'deployment status never finalized'
		 WHERE outcome = 'in_progress' AND deployed_at < NOW() - make_interval(secs => $1)`,
		olderThan.Seconds(),
	)
	if err != nil {
		return fmt.Errorf("mark stale in_progress: %w", err)
	}
	return nil
}

func scanActivityDeployments(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.Deployment, error) {
	var result []domain.Deployment
	for rows.Next() {
		var d domain.Deployment
		if err := rows.Scan(
			&d.ID, &d.ProductID, &d.ProductSlug, &d.EnvironmentID, &d.ActorDisplayName, &d.ComponentName,
			&d.EnvironmentName, &d.Tag, &d.DeployedAt, &d.CommitSHA, &d.Outcome, &d.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("deployment store scan: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("deployment store rows: %w", err)
	}
	return result, nil
}

func scanDeployments(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.Deployment, error) {
	var result []domain.Deployment
	for rows.Next() {
		var d domain.Deployment
		if err := rows.Scan(
			&d.ID, &d.ProductID, &d.EnvironmentID, &d.ActorDisplayName, &d.ComponentName,
			&d.EnvironmentName, &d.Tag, &d.DeployedAt, &d.CommitSHA, &d.Outcome, &d.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("deployment store scan: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("deployment store rows: %w", err)
	}
	return result, nil
}

// isInvalidUUIDSyntax reports whether err is a PostgreSQL invalid text representation
// error for a UUID column (SQLSTATE 22P02). This occurs when a non-UUID string is passed
// as a UUID parameter, which should be treated as not-found rather than a server error.
func isInvalidUUIDSyntax(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
