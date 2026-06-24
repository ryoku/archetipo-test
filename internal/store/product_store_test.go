package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

func getProductTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip(skipDatabaseTestMessage)
	}
	return dsn
}

func newProductTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := getProductTestDSN(t)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanProducts(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "DELETE FROM products")
	if err != nil {
		t.Fatalf("cleanProducts: %v", err)
	}
}

func TestProductStore_Create(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{
		Name:        "Test Product",
		Slug:        "test-product",
		Description: "A product for testing",
	}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Error("expected ID to be set after Create")
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set after Create")
	}
}

func TestProductStore_Create_SlugConflict(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p1 := &domain.Product{Name: "Product A", Slug: "conflict-slug"}
	if err := s.Create(context.Background(), p1); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	p2 := &domain.Product{Name: "Product B", Slug: "conflict-slug"}
	err := s.Create(context.Background(), p2)
	if err == nil {
		t.Fatal("expected error for duplicate slug, got nil")
	}
	if err != store.ErrSlugConflict {
		t.Errorf("expected ErrSlugConflict, got %v", err)
	}
}

func TestProductStore_List_FiltersBySlug(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	for _, p := range []*domain.Product{
		{Name: "Alpha", Slug: "alpha"},
		{Name: "Beta", Slug: "beta"},
	} {
		if err := s.Create(context.Background(), p); err != nil {
			t.Fatalf("Create %s: %v", p.Slug, err)
		}
	}

	all, err := s.List(context.Background(), store.ListOptions{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 products, got %d", len(all))
	}

	filtered, err := s.List(context.Background(), store.ListOptions{
		SlugAllowlist: map[string]struct{}{"alpha": {}},
	})
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Slug != "alpha" {
		t.Errorf("expected only alpha, got %+v", filtered)
	}
}

func TestProductStore_List_ExcludesArchivedByDefault(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Archived", Slug: "archived-product"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Archive(context.Background(), "archived-product"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	all, err := s.List(context.Background(), store.ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, prod := range all {
		if prod.Slug == "archived-product" {
			t.Error("archived product should not appear in default listing")
		}
	}

	withArchived, err := s.List(context.Background(), store.ListOptions{IncludeArchived: true})
	if err != nil {
		t.Fatalf("List with archived: %v", err)
	}
	found := false
	for _, prod := range withArchived {
		if prod.Slug == "archived-product" {
			found = true
		}
	}
	if !found {
		t.Error("archived product should appear when IncludeArchived=true")
	}
}

func TestProductStore_GetBySlug(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Findable", Slug: "findable", Description: "desc"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.GetBySlug(context.Background(), "findable")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.Name != "Findable" {
		t.Errorf("expected Name=Findable, got %q", got.Name)
	}

	_, err = s.GetBySlug(context.Background(), "nonexistent")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for nonexistent slug, got %v", err)
	}
}

func TestProductStore_Update(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Original", Slug: "original", Description: "old"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := s.Update(context.Background(), "original", "Updated Name", "new desc")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("expected Name=Updated Name, got %q", updated.Name)
	}
	if updated.Slug != "original" {
		t.Errorf("slug must not change, got %q", updated.Slug)
	}

	_, err = s.Update(context.Background(), "nonexistent", "x", "y")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProductStore_Archive(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "To Archive", Slug: "to-archive"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Archive(context.Background(), "to-archive"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	got, err := s.GetBySlug(context.Background(), "to-archive")
	if err != nil {
		t.Fatalf("GetBySlug after archive: %v", err)
	}
	if got.ArchivedAt == nil {
		t.Error("expected ArchivedAt to be set after archive")
	}
}

func TestProductStore_Archive_NotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	err := s.Archive(context.Background(), "nonexistent")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProductStore_List_EmptyAllowlist(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Product", Slug: "some-product"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := s.List(context.Background(), store.ListOptions{
		SlugAllowlist: map[string]struct{}{},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty allowlist, got %d", len(results))
	}
}

func TestGetTagConvention_NoOverride(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "No Convention", Slug: "no-convention"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.GetTagConvention(context.Background(), "no-convention")
	if err != nil {
		t.Fatalf("GetTagConvention: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil pointer for product with no override, got %q", *got)
	}
}

func TestSetAndGetTagConvention(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Convention Product", Slug: "convention-product"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	regex := `^v\d+\.\d+\.\d+$`
	if err := s.SetTagConvention(context.Background(), "convention-product", regex); err != nil {
		t.Fatalf("SetTagConvention: %v", err)
	}

	got, err := s.GetTagConvention(context.Background(), "convention-product")
	if err != nil {
		t.Fatalf("GetTagConvention: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil pointer after SetTagConvention, got nil")
		return
	}
	if *got != regex {
		t.Errorf("expected regex %q, got %q", regex, *got)
	}
}

func TestSetTagConvention_NotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	err := s.SetTagConvention(context.Background(), "nonexistent", `^v\d+$`)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent slug, got %v", err)
	}
}

func TestGetTagConvention_NotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	_, err := s.GetTagConvention(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent slug, got %v", err)
	}
}

func TestGetTagConvention_ArchivedProduct_ReturnsNotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Archived Product", Slug: "archived-get-conv"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Archive(context.Background(), "archived-get-conv"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	_, err := s.GetTagConvention(context.Background(), "archived-get-conv")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for archived product, got %v", err)
	}
}

func TestSetTagConvention_ArchivedProduct_ReturnsNotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Archived Product", Slug: "archived-set-conv"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Archive(context.Background(), "archived-set-conv"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	err := s.SetTagConvention(context.Background(), "archived-set-conv", `^v\d+$`)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for archived product, got %v", err)
	}
}

func TestClearTagConvention_RoundTrip(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Clear Product", Slug: "clear-convention"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.SetTagConvention(context.Background(), "clear-convention", `^v\d+$`); err != nil {
		t.Fatalf("SetTagConvention: %v", err)
	}

	if err := s.ClearTagConvention(context.Background(), "clear-convention"); err != nil {
		t.Fatalf("ClearTagConvention: %v", err)
	}

	got, err := s.GetTagConvention(context.Background(), "clear-convention")
	if err != nil {
		t.Fatalf("GetTagConvention after clear: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after clear, got %q", *got)
	}
}

func TestClearTagConvention_NotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	err := s.ClearTagConvention(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent slug, got %v", err)
	}
}

func TestClearTagConvention_ArchivedProduct_ReturnsNotFound(t *testing.T) {
	pool := newProductTestPool(t)
	cleanProducts(t, pool)

	s := store.NewProductStore(pool)
	p := &domain.Product{Name: "Archived Product", Slug: "archived-clear-conv"}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Archive(context.Background(), "archived-clear-conv"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	err := s.ClearTagConvention(context.Background(), "archived-clear-conv")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for archived product, got %v", err)
	}
}

func cleanEnvsAndProducts(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "DELETE FROM environments"); err != nil {
		t.Fatalf("delete environments: %v", err)
	}
	cleanProducts(t, pool)
}

func TestProductStore_ListWithStats_EmptyList(t *testing.T) {
	ctx := context.Background()
	pool := newProductTestPool(t)
	cleanEnvsAndProducts(t, pool)
	s := store.NewProductStore(pool)

	results, err := s.ListWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWithStats: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty table, got %d", len(results))
	}
}

func TestProductStore_ListWithStats_NoEnvironments(t *testing.T) {
	ctx := context.Background()
	pool := newProductTestPool(t)
	cleanEnvsAndProducts(t, pool)
	s := store.NewProductStore(pool)

	p := &domain.Product{Name: "No Envs", Slug: "no-envs"}
	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := s.ListWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWithStats: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].EnvironmentCount != 0 {
		t.Errorf("expected EnvironmentCount=0, got %d", results[0].EnvironmentCount)
	}
	if results[0].LastDeployedAt != nil {
		t.Errorf("expected LastDeployedAt=nil, got %v", results[0].LastDeployedAt)
	}
}

func TestProductStore_ListWithStats_TwoEnvironments(t *testing.T) {
	ctx := context.Background()
	pool := newProductTestPool(t)
	cleanEnvsAndProducts(t, pool)
	s := store.NewProductStore(pool)

	p := &domain.Product{Name: "Two Envs", Slug: "two-envs"}
	if err := s.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, env := range []struct{ name, slug, typ string }{
		{"Dev", "dev", "dev"},
		{"Integration", "integration", "integration"},
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO environments (product_id, name, slug, type) VALUES ($1, $2, $3, $4)`,
			p.ID, env.name, env.slug, env.typ,
		); err != nil {
			t.Fatalf("insert environment %s: %v", env.name, err)
		}
	}

	results, err := s.ListWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWithStats: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].EnvironmentCount != 2 {
		t.Errorf("expected EnvironmentCount=2, got %d", results[0].EnvironmentCount)
	}
	if results[0].LastDeployedAt != nil {
		t.Errorf("expected LastDeployedAt=nil (no deployments inserted), got %v", results[0].LastDeployedAt)
	}
}

func TestProductStore_ListWithStats_ArchivedExcluded(t *testing.T) {
	ctx := context.Background()
	pool := newProductTestPool(t)
	cleanEnvsAndProducts(t, pool)
	s := store.NewProductStore(pool)

	active := &domain.Product{Name: "Active", Slug: "active-stats"}
	archived := &domain.Product{Name: "Archived", Slug: "archived-stats"}
	if err := s.Create(ctx, active); err != nil {
		t.Fatalf("Create active: %v", err)
	}
	if err := s.Create(ctx, archived); err != nil {
		t.Fatalf("Create archived: %v", err)
	}
	if err := s.Archive(ctx, "archived-stats"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	results, err := s.ListWithStats(ctx)
	if err != nil {
		t.Fatalf("ListWithStats: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only active), got %d", len(results))
	}
	if results[0].Slug != "active-stats" {
		t.Errorf("expected active-stats in results, got %q", results[0].Slug)
	}
}
