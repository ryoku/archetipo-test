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
	// deployments reference environments without ON DELETE CASCADE, so clear them first.
	if _, err := pool.Exec(context.Background(), "DELETE FROM deployments"); err != nil {
		t.Fatalf("cleanEnvironments (deployments): %v", err)
	}
	if _, err := pool.Exec(context.Background(), "DELETE FROM environments"); err != nil {
		t.Fatalf("cleanEnvironments: %v", err)
	}
}

// createTestProduct creates a product in the database for use as a parent in environment tests.
// Moved from component_store_test.go where it was originally shared.
func createTestProduct(t *testing.T, pool *pgxpool.Pool, slugSuffix string) *domain.Product {
	t.Helper()
	ps := store.NewProductStore(pool)
	p := &domain.Product{
		Name:        "Test Product " + slugSuffix,
		Slug:        "test-product-" + slugSuffix,
		Description: "created for store tests",
	}
	if err := ps.Create(context.Background(), p); err != nil {
		t.Fatalf("createTestProduct: %v", err)
	}
	return p
}

// newEnvStoreTest opens a pool, clears state, creates a product, and returns a
// ready EnvironmentStore and the product. The pool is closed via t.Cleanup.
func newEnvStoreTest(t *testing.T, productSlug string) (store.EnvironmentStore, *domain.Product) {
	t.Helper()
	pool := newEnvironmentTestPool(t)
	cleanEnvironments(t, pool)
	cleanProducts(t, pool)
	prod := createTestProduct(t, pool, productSlug)
	return store.NewEnvironmentStore(pool), prod
}

func TestEnvironmentStore_Create(t *testing.T) {
	es, prod := newEnvStoreTest(t, "env-create")

	e := &domain.Environment{
		ProductID: prod.ID,
		Name:      "dev",
		Type:      "dev",
		Slug:      "dev",
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
	es, prod := newEnvStoreTest(t, "env-list")

	envs := []domain.Environment{
		{ProductID: prod.ID, Name: "dev", Type: "dev", Slug: "dev"},
		{ProductID: prod.ID, Name: "integration", Type: "integration", Slug: "integration"},
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
	es, prod := newEnvStoreTest(t, "env-delete")

	e := &domain.Environment{
		ProductID: prod.ID,
		Name:      "dev",
		Type:      "dev",
		Slug:      "dev",
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
	es, prod := newEnvStoreTest(t, "env-delete-notfound")

	err := es.Delete(context.Background(), prod.ID, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent environment, got nil")
	}
	if err != store.ErrEnvironmentNotFound {
		t.Errorf("expected ErrEnvironmentNotFound, got %v", err)
	}
}

func assertEnvConflict(t *testing.T, es store.EnvironmentStore, e *domain.Environment, wantErr error) {
	t.Helper()
	err := es.Create(context.Background(), e)
	if err == nil {
		t.Fatalf("expected %v, got nil", wantErr)
	}
	if err != wantErr {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

func TestEnvironmentStore_Create_NameConflict(t *testing.T) {
	es, prod := newEnvStoreTest(t, "env-conflict")

	if err := es.Create(context.Background(), &domain.Environment{
		ProductID: prod.ID,
		Name:      "dev",
		Type:      "dev",
		Slug:      "dev",
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	assertEnvConflict(t, es, &domain.Environment{
		ProductID: prod.ID,
		Name:      "dev",
		Type:      "dev",
		Slug:      "dev-2",
	}, store.ErrEnvironmentNameConflict)
}

func TestEnvironmentStore_Create_SlugConflict(t *testing.T) {
	es, prod := newEnvStoreTest(t, "env-slug-conflict")

	if err := es.Create(context.Background(), &domain.Environment{
		ProductID: prod.ID,
		Name:      "dev-env",
		Type:      "dev",
		Slug:      "dev",
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	assertEnvConflict(t, es, &domain.Environment{
		ProductID: prod.ID,
		Name:      "dev-environment",
		Type:      "dev",
		Slug:      "dev",
	}, store.ErrEnvironmentSlugConflict)
}
