package main

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port

	r := gin.Default()
	registerSPA(r)

	log.Printf("Server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// pgx5DSN converts a standard postgresql:// or postgres:// URL to the pgx5://
// scheme expected by golang-migrate's pgx/v5 driver.
func pgx5DSN(dsn string) string {
	dsn = strings.Replace(dsn, "postgresql://", "pgx5://", 1)
	dsn = strings.Replace(dsn, "postgres://", "pgx5://", 1)
	return dsn
}

func runMigrations(dsn string) error {
	m, err := migrate.New("file://migrations", pgx5DSN(dsn))
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
