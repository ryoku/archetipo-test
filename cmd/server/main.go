package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/api/router"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	issuerURL := os.Getenv("OIDC_ISSUER_URL")
	if issuerURL == "" {
		kcURL := strings.TrimRight(os.Getenv("KEYCLOAK_URL"), "/")
		kcRealm := strings.Trim(os.Getenv("KEYCLOAK_REALM"), "/")
		if kcURL == "" || kcRealm == "" {
			log.Fatal("set OIDC_ISSUER_URL, or both KEYCLOAK_URL and KEYCLOAK_REALM")
		}
		issuerURL = kcURL + "/realms/" + kcRealm
	}

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	clientID := os.Getenv("OIDC_CLIENT_ID") // optional; enables aud claim validation when set

	oidcCtx, oidcCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer oidcCancel()
	verifier, err := auth.NewVerifier(oidcCtx, issuerURL, clientID)
	if err != nil {
		log.Fatalf("OIDC verifier init: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	productStore := store.NewProductStore(pool)
	componentStore := store.NewComponentStore(pool)
	environmentStore := store.NewEnvironmentStore(pool)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port

	r := router.New(verifier,
		router.RegisterProductRoutes(productStore),
		router.RegisterComponentRoutes(productStore, componentStore),
		router.RegisterEnvironmentRoutes(productStore, environmentStore),
	)
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
