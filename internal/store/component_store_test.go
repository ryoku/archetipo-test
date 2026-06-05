package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

func newComponentTestPool(t *testing.T) *pgxpool.Pool {
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

func cleanComponents(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "DELETE FROM components")
	if err != nil {
		t.Fatalf("cleanComponents: %v", err)
	}
}


func createTestProduct(t *testing.T, pool *pgxpool.Pool, slugSuffix string) *domain.Product {
	t.Helper()
	ps := store.NewProductStore(pool)
	p := &domain.Product{
		Name:        "Test Product " + slugSuffix,
		Slug:        "test-product-" + slugSuffix,
		Description: "created for component tests",
	}
	if err := ps.Create(context.Background(), p); err != nil {
		t.Fatalf("createTestProduct: %v", err)
	}
	return p
}

func TestComponentStore_Create_Valid(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	suffix := fmt.Sprintf("%d", 1)
	prod := createTestProduct(t, pool, suffix)
	cs := store.NewComponentStore(pool)

	comp := &domain.Component{
		ProductID:    prod.ID,
		Name:         "My Component",
		Slug:         "my-component",
		GCRImagePath: "gcr.io/project/image",
	}
	if err := cs.Create(context.Background(), comp); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if comp.ID == "" {
		t.Error("expected ID to be set after Create")
	}
	if comp.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set after Create")
	}
}

func TestComponentStore_Create_SlugConflict(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "conflict")
	cs := store.NewComponentStore(pool)

	c1 := &domain.Component{
		ProductID: prod.ID, Name: "Comp A", Slug: "dup-slug", GCRImagePath: "gcr.io/p/a",
	}
	if err := cs.Create(context.Background(), c1); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	c2 := &domain.Component{
		ProductID: prod.ID, Name: "Comp B", Slug: "dup-slug", GCRImagePath: "gcr.io/p/b",
	}
	err := cs.Create(context.Background(), c2)
	if err == nil {
		t.Fatal("expected error for duplicate (productID, slug), got nil")
	}
	if err != store.ErrComponentSlugConflict {
		t.Errorf("expected ErrComponentSlugConflict, got %v", err)
	}
}

func TestComponentStore_Create_SameSlugDifferentProduct(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	prod1 := createTestProduct(t, pool, "p1")
	prod2 := createTestProduct(t, pool, "p2")
	cs := store.NewComponentStore(pool)

	c1 := &domain.Component{
		ProductID: prod1.ID, Name: "Comp", Slug: "shared-slug", GCRImagePath: "gcr.io/p/img",
	}
	if err := cs.Create(context.Background(), c1); err != nil {
		t.Fatalf("Create for product1: %v", err)
	}

	c2 := &domain.Component{
		ProductID: prod2.ID, Name: "Comp", Slug: "shared-slug", GCRImagePath: "gcr.io/p/img",
	}
	if err := cs.Create(context.Background(), c2); err != nil {
		t.Errorf("expected success for same slug on different product, got %v", err)
	}
}

func TestComponentStore_ListByProduct_Empty(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "empty")
	cs := store.NewComponentStore(pool)

	comps, err := cs.ListByProduct(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if comps == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(comps) != 0 {
		t.Errorf("expected 0 components, got %d", len(comps))
	}
}

func TestComponentStore_ListByProduct_WithItems(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "withitems")
	cs := store.NewComponentStore(pool)

	for i, slug := range []string{"comp-a", "comp-b"} {
		c := &domain.Component{
			ProductID:    prod.ID,
			Name:         fmt.Sprintf("Component %d", i+1),
			Slug:         slug,
			GCRImagePath: "gcr.io/proj/" + slug,
		}
		if err := cs.Create(context.Background(), c); err != nil {
			t.Fatalf("Create %s: %v", slug, err)
		}
	}

	comps, err := cs.ListByProduct(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if len(comps) != 2 {
		t.Errorf("expected 2 components, got %d", len(comps))
	}
}

func TestComponentStore_Delete_Success(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "del")
	cs := store.NewComponentStore(pool)

	comp := &domain.Component{
		ProductID: prod.ID, Name: "To Delete", Slug: "to-delete", GCRImagePath: "gcr.io/p/img",
	}
	if err := cs.Create(context.Background(), comp); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := cs.Delete(context.Background(), prod.ID, "to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify gone
	comps, err := cs.ListByProduct(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("ListByProduct after delete: %v", err)
	}
	if len(comps) != 0 {
		t.Errorf("expected 0 components after delete, got %d", len(comps))
	}
}

func TestComponentStore_Delete_NotFound(t *testing.T) {
	pool := newComponentTestPool(t)
	cleanComponents(t, pool)
	cleanProducts(t, pool)

	prod := createTestProduct(t, pool, "notfound")
	cs := store.NewComponentStore(pool)

	err := cs.Delete(context.Background(), prod.ID, "nonexistent-slug")
	if err == nil {
		t.Fatal("expected error when deleting nonexistent component, got nil")
	}
	if err != store.ErrComponentNotFound {
		t.Errorf("expected ErrComponentNotFound, got %v", err)
	}
}
