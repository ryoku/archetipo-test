package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

func newEnvironmentTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip(skipDatabaseTestMessage)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanEnvironments(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "DELETE FROM environments")
	if err != nil {
		t.Fatalf("cleanEnvironments: %v", err)
	}
}

func TestEnvironmentStore_Create(t *testing.T) {
	pool := newEnvironmentTestPool(t)
	cleanEnvironments(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "env-create")
	es := store.NewEnvironmentStore(pool)

	e := &domain.Environment{
		ProductID:   prod.ID,
		Name:        "dev",
		Type:        "dev",
		OverlayPath: "overlays/dev",
	}
	if err := es.Create(context.Background(), e); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == "" {
		t.Error("expected ID to be set after Create")
	}
	if e.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set after Create")
	}
}

func TestEnvironmentStore_ListByProduct(t *testing.T) {
	pool := newEnvironmentTestPool(t)
	cleanEnvironments(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "env-list")
	es := store.NewEnvironmentStore(pool)

	envs := []domain.Environment{
		{ProductID: prod.ID, Name: "dev", Type: "dev", OverlayPath: "overlays/dev"},
		{ProductID: prod.ID, Name: "integration", Type: "integration", OverlayPath: "overlays/integration"},
	}
	for i := range envs {
		if err := es.Create(context.Background(), &envs[i]); err != nil {
			t.Fatalf("Create environment %q: %v", envs[i].Name, err)
		}
	}

	result, err := es.ListByProduct(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 environments, got %d", len(result))
	}
	// Results are ordered by created_at ASC; first inserted should be first.
	if result[0].Name != "dev" {
		t.Errorf("expected first environment name=dev, got %q", result[0].Name)
	}
	if result[1].Name != "integration" {
		t.Errorf("expected second environment name=integration, got %q", result[1].Name)
	}
}

func TestEnvironmentStore_Delete(t *testing.T) {
	pool := newEnvironmentTestPool(t)
	cleanEnvironments(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "env-delete")
	es := store.NewEnvironmentStore(pool)

	e := &domain.Environment{
		ProductID:   prod.ID,
		Name:        "dev",
		Type:        "dev",
		OverlayPath: "overlays/dev",
	}
	if err := es.Create(context.Background(), e); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := es.Delete(context.Background(), prod.ID, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	result, err := es.ListByProduct(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("ListByProduct after delete: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 environments after delete, got %d", len(result))
	}
}

func TestEnvironmentStore_Delete_NotFound(t *testing.T) {
	pool := newEnvironmentTestPool(t)
	cleanEnvironments(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "env-delete-notfound")
	es := store.NewEnvironmentStore(pool)

	err := es.Delete(context.Background(), prod.ID, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent environment, got nil")
	}
	if err != store.ErrEnvironmentNotFound {
		t.Errorf("expected ErrEnvironmentNotFound, got %v", err)
	}
}

func TestEnvironmentStore_Create_NameConflict(t *testing.T) {
	pool := newEnvironmentTestPool(t)
	cleanEnvironments(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "env-conflict")
	es := store.NewEnvironmentStore(pool)

	e1 := &domain.Environment{
		ProductID:   prod.ID,
		Name:        "dev",
		Type:        "dev",
		OverlayPath: "overlays/dev",
	}
	if err := es.Create(context.Background(), e1); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	e2 := &domain.Environment{
		ProductID:   prod.ID,
		Name:        "dev",
		Type:        "dev",
		OverlayPath: "overlays/dev-2",
	}
	err := es.Create(context.Background(), e2)
	if err == nil {
		t.Fatal("expected error for duplicate environment name in same product, got nil")
	}
	if err != store.ErrEnvironmentNameConflict {
		t.Errorf("expected ErrEnvironmentNameConflict, got %v", err)
	}
}
