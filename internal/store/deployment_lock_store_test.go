package store_test

import (
	"context"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestDeploymentLockStore_AcquireRelease(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewDeploymentLockStore(pool)

	productID, envID := insertLockFixtures(t, pool)

	lock, holder, err := s.TryAcquire(context.Background(), productID, envID, "sara", 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock)
	assert.Nil(t, holder)

	info, err := s.GetLockInfo(context.Background(), productID, envID)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "sara", info.LockHolder)
	assert.WithinDuration(t, time.Now(), info.LockedSince, 5*time.Second)

	require.NoError(t, lock.Release(context.Background()))

	info, err = s.GetLockInfo(context.Background(), productID, envID)
	require.NoError(t, err)
	assert.Nil(t, info)
}

func TestDeploymentLockStore_ContendedLock(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewDeploymentLockStore(pool)

	productID, envID := insertLockFixtures(t, pool)

	// First caller acquires the lock.
	lock1, _, err := s.TryAcquire(context.Background(), productID, envID, "sara", 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock1)

	// Second caller cannot acquire within 200 ms.
	lock2, holder, err := s.TryAcquire(context.Background(), productID, envID, "marco", 200*time.Millisecond)
	require.NoError(t, err)
	assert.Nil(t, lock2)
	require.NotNil(t, holder)
	assert.Equal(t, "sara", holder.LockHolder)

	// After first releases, a new caller succeeds.
	require.NoError(t, lock1.Release(context.Background()))

	lock3, _, err := s.TryAcquire(context.Background(), productID, envID, "marco", 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock3)
	require.NoError(t, lock3.Release(context.Background()))
}

func TestDeploymentLockStore_AutoReleaseOnConnectionClose(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewDeploymentLockStore(pool)

	productID, envID := insertLockFixtures(t, pool)

	// Acquire lock via a separate short-lived pool to simulate a dropped connection.
	pool2, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	require.NoError(t, err)
	s2 := store.NewDeploymentLockStore(pool2)

	lock, _, err := s2.TryAcquire(context.Background(), productID, envID, "sara", 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock)

	// Close the pool without an orderly Release (simulates server crash / connection drop).
	// PostgreSQL automatically releases the session-level advisory lock when the session ends.
	pool2.Close()

	// The metadata row is left orphaned by the crash; delete it manually here to simulate
	// the normal acquire path overwriting it via ON CONFLICT DO UPDATE on the next attempt.
	_, _ = pool.Exec(context.Background(),
		`DELETE FROM deployment_locks WHERE product_id = $1 AND env_id = $2`, productID, envID)

	// A new caller should now be able to acquire the lock.
	lock3, _, err := s.TryAcquire(context.Background(), productID, envID, "marco", 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock3)
	require.NoError(t, lock3.Release(context.Background()))
}

func TestDeploymentLockStore_ConcurrentAcquire(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewDeploymentLockStore(pool)

	productID, envID := insertLockFixtures(t, pool)

	const workers = 5
	results := make([]bool, workers)
	var wg sync.WaitGroup
	var concurrent int64
	var violated int64

	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lock, _, err := s.TryAcquire(context.Background(), productID, envID, "worker", 300*time.Millisecond)
			if err == nil && lock != nil {
				results[idx] = true
				if atomic.AddInt64(&concurrent, 1) > 1 {
					atomic.StoreInt64(&violated, 1)
				}
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt64(&concurrent, -1)
				_ = lock.Release(context.Background())
			}
		}(i)
	}
	wg.Wait()

	acquiredCount := 0
	for _, r := range results {
		if r {
			acquiredCount++
		}
	}
	assert.GreaterOrEqual(t, acquiredCount, 1)
	assert.Equal(t, int64(0), atomic.LoadInt64(&violated), "multiple goroutines held the lock simultaneously")
}

// insertLockFixtures creates a product and environment row for lock tests and returns their IDs.
func insertLockFixtures(t *testing.T, pool *pgxpool.Pool) (productID, envID string) {
	t.Helper()
	ctx := context.Background()

	err := pool.QueryRow(ctx,
		`INSERT INTO products (name, slug, description) VALUES ($1, $2, $3) RETURNING id`,
		"Lock Test Product "+t.Name(), sanitizeSlug(t.Name()), "",
	).Scan(&productID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO environments (product_id, name, slug, type, overlay_path) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		productID, "dev", "dev", "dev", "apps/dev/lock-test/lock-test-helmrelease.yaml",
	).Scan(&envID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM deployment_locks WHERE product_id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM environments WHERE product_id = $1`, productID)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID)
	})
	return productID, envID
}

// sanitizeSlug converts a test name into a URL-safe slug fragment.
func sanitizeSlug(name string) string {
	// Map each character to lowercase alnum or '-'.
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '-'
		}
	}, name)

	// Collapse runs of hyphens and trim leading/trailing ones.
	slug := strings.Trim(multiHyphen.ReplaceAllString(mapped, "-"), "-")

	if slug == "" {
		return "test"
	}
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	return slug
}

var multiHyphen = regexp.MustCompile(`-{2,}`)
