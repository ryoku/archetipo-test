package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// pgx5DSN converts a standard postgresql:// or postgres:// URL to the pgx5://
// scheme expected by golang-migrate's pgx/v5 driver.
func pgx5DSN(dsn string) string {
	dsn = strings.Replace(dsn, "postgresql://", "pgx5://", 1)
	dsn = strings.Replace(dsn, "postgres://", "pgx5://", 1)
	return dsn
}

// currentVersion returns the current migration version, or 0 if none applied yet.
func currentVersion(m *migrate.Migrate) uint {
	v, _, err := m.Version()
	if err != nil {
		return 0
	}
	return v
}

// versionErr returns only the error from m.Version(), used for nil-version checks.
func versionErr(m *migrate.Migrate) error {
	_, _, err := m.Version()
	return err
}

func main() {
	direction := flag.String("direction", "up", "Migration direction: up or down")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	m, err := migrate.New("file://migrations", pgx5DSN(dsn))
	if err != nil {
		log.Fatalf("Failed to initialise migration: %v", err)
	}
	defer m.Close()

	switch *direction {
	case "up":
		versionBefore := currentVersion(m)
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration up failed: %v", err)
		}
		versionAfter := currentVersion(m)
		applied := int(versionAfter) - int(versionBefore)
		if applied == 0 {
			fmt.Println("No pending migrations.")
		} else {
			fmt.Printf("Applied %d migration(s). Current version: %d\n", applied, versionAfter)
		}
	case "down":
		if errors.Is(versionErr(m), migrate.ErrNilVersion) {
			fmt.Println("Nothing to roll back.")
			return
		}
		if err := m.Steps(-1); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
		fmt.Printf("Rolled back 1 migration. Current version: %d\n", currentVersion(m))
	default:
		log.Fatalf("Unknown direction %q — use 'up' or 'down'", *direction)
	}
}
