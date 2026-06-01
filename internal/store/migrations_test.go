package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
)

func newMigrate(t *testing.T, dsn string) *migrate.Migrate {
	t.Helper()
	m, err := migrate.New("file://../../migrations", dsn)
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	return m
}

func TestMigrations_UpCreatesProductsTable(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	m := newMigrate(t, dsn)
	// ensure clean state
	_ = m.Down()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("m.Up(): %v", err)
	}

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgx.Connect: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	expectedColumns := []string{"id", "name", "slug", "description", "archived_at", "created_at"}
	for _, col := range expectedColumns {
		var exists bool
		err := conn.QueryRow(context.Background(),
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'products' AND column_name = $1
			)`, col).Scan(&exists)
		if err != nil {
			t.Fatalf("querying column %q: %v", col, err)
		}
		if !exists {
			t.Errorf("column %q not found in products table", col)
		}
	}
}

func TestMigrations_UpIsIdempotent(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	m := newMigrate(t, dsn)
	_ = m.Down()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("first Up: %v", err)
	}

	m2 := newMigrate(t, dsn)
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Errorf("second Up should return ErrNoChange, got: %v", err)
	}
}

func TestMigrations_DownDropsProductsTable(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	m := newMigrate(t, dsn)
	_ = m.Down()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("Up before Down test: %v", err)
	}

	m2 := newMigrate(t, dsn)
	if err := m2.Steps(-1); err != nil {
		t.Fatalf("Steps(-1): %v", err)
	}

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgx.Connect: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	var exists bool
	err = conn.QueryRow(context.Background(),
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'products'
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("querying table existence: %v", err)
	}
	if exists {
		t.Error("products table still exists after migration down")
	}
}

func TestMigrations_InvalidDSNReturnsError(t *testing.T) {
	_, err := migrate.New("file://../../migrations", "postgresql://invalid:invalid@localhost:9999/nonexistent")
	if err == nil {
		t.Error("expected error for invalid DSN, got nil")
	}
}
