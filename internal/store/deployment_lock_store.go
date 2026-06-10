package store

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// ErrDeploymentLockHeld is returned when an advisory lock cannot be acquired because another
// deployment is already in progress for the same product-environment pair.
var ErrDeploymentLockHeld = errors.New("deployment lock held by another process")

// AcquiredLock represents a held deployment advisory lock. Release must be called exactly once.
type AcquiredLock interface {
	Release(ctx context.Context) error
}

// DeploymentLockStore serializes deployments per product-environment pair via PostgreSQL
// session-level advisory locks backed by a metadata table.
type DeploymentLockStore interface {
	// TryAcquire attempts to acquire the advisory lock for the given product-environment pair.
	// It polls pg_try_advisory_lock every 100 ms until the lock is acquired or the timeout elapses.
	// Returns (non-nil AcquiredLock, nil, nil) on success.
	// Returns (nil, holderInfo, nil) on contention after timeout.
	// Returns (nil, nil, err) on a technical error.
	TryAcquire(ctx context.Context, productID, envID, actor string, timeout time.Duration) (AcquiredLock, *domain.DeploymentLock, error)

	// GetLockInfo returns current lock metadata. Returns (nil, nil) when no lock is held.
	GetLockInfo(ctx context.Context, productID, envID string) (*domain.DeploymentLock, error)
}

type pgxDeploymentLockStore struct {
	pool *pgxpool.Pool
}

// NewDeploymentLockStore returns a DeploymentLockStore backed by the given pgxpool.
func NewDeploymentLockStore(pool *pgxpool.Pool) DeploymentLockStore {
	return &pgxDeploymentLockStore{pool: pool}
}

// lockKeys derives two int32 advisory lock arguments from product and environment UUIDs using FNV-1a.
func lockKeys(productID, envID string) (int32, int32) {
	h1 := fnv.New32a()
	h1.Write([]byte(productID)) //nolint:errcheck // hash.Hash.Write never returns an error
	h2 := fnv.New32a()
	h2.Write([]byte(envID)) //nolint:errcheck
	return int32(h1.Sum32()), int32(h2.Sum32())
}

func (s *pgxDeploymentLockStore) TryAcquire(ctx context.Context, productID, envID, actor string, timeout time.Duration) (AcquiredLock, *domain.DeploymentLock, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("acquire connection: %w", err)
	}

	k1, k2 := lockKeys(productID, envID)
	deadline := time.Now().Add(timeout)

	for {
		var acquired bool
		if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1, $2)", k1, k2).Scan(&acquired); err != nil {
			conn.Release()
			return nil, nil, fmt.Errorf("advisory lock attempt: %w", err)
		}
		if acquired {
			if _, err := conn.Exec(ctx,
				`INSERT INTO deployment_locks (product_id, env_id, lock_holder, locked_since)
				 VALUES ($1, $2, $3, NOW())
				 ON CONFLICT (product_id, env_id) DO UPDATE
				     SET lock_holder = EXCLUDED.lock_holder, locked_since = NOW()`,
				productID, envID, actor,
			); err != nil {
				_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1, $2)", k1, k2)
				conn.Release()
				return nil, nil, fmt.Errorf("insert lock metadata: %w", err)
			}
			lock := &pgxAcquiredLock{conn: conn, store: s, productID: productID, envID: envID}
			return lock, nil, nil
		}

		if time.Now().After(deadline) {
			conn.Release()
			info, err := s.GetLockInfo(ctx, productID, envID)
			return nil, info, err
		}

		select {
		case <-ctx.Done():
			conn.Release()
			return nil, nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *pgxDeploymentLockStore) GetLockInfo(ctx context.Context, productID, envID string) (*domain.DeploymentLock, error) {
	var lock domain.DeploymentLock
	err := s.pool.QueryRow(ctx,
		`SELECT product_id, env_id, lock_holder, locked_since
		 FROM deployment_locks
		 WHERE product_id = $1 AND env_id = $2`,
		productID, envID,
	).Scan(&lock.ProductID, &lock.EnvID, &lock.LockHolder, &lock.LockedSince)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get lock info: %w", err)
	}
	return &lock, nil
}

// pgxAcquiredLock holds the dedicated connection that owns the session-level advisory lock.
type pgxAcquiredLock struct {
	conn      *pgxpool.Conn
	store     *pgxDeploymentLockStore
	productID string
	envID     string
}

func (l *pgxAcquiredLock) Release(ctx context.Context) error {
	defer l.conn.Release()
	if _, err := l.conn.Exec(ctx,
		`DELETE FROM deployment_locks WHERE product_id = $1 AND env_id = $2`,
		l.productID, l.envID,
	); err != nil {
		return fmt.Errorf("delete lock metadata: %w", err)
	}
	k1, k2 := lockKeys(l.productID, l.envID)
	if _, err := l.conn.Exec(ctx, "SELECT pg_advisory_unlock($1, $2)", k1, k2); err != nil {
		return fmt.Errorf("advisory unlock: %w", err)
	}
	return nil
}
