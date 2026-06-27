package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/api/router"
	"github.com/ryoku/kubegate/internal/auth"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gcr"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	issuerURL, err := resolveIssuerURL()
	if err != nil {
		log.Fatal(err)
	}

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	clientID := os.Getenv("OIDC_CLIENT_ID")

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
	environmentStore := store.NewEnvironmentStore(pool)
	lockStore := store.NewDeploymentLockStore(pool)
	deploymentStore := store.NewDeploymentStore(pool)
	statsStore := store.NewStatsStore(pool)
	tagConventionDefault := os.Getenv("TAG_CONVENTION_DEFAULT")

	gcrLister, closeGCR := initGCRLister()
	defer closeGCR()
	gitopsApplier, workloadReader, statusReader := initGitOps()

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081"
	}

	r := router.New(verifier,
		router.RegisterProductRoutes(productStore),
		router.RegisterEnvironmentRoutes(productStore, environmentStore),
		router.RegisterTagConventionRoutes(productStore, tagConventionDefault),
		router.RegisterWorkloadRoutes(productStore, environmentStore, workloadReader),
		router.RegisterTagRoutes(productStore, environmentStore, workloadReader, gcrLister),
		router.RegisterDeploymentRoutes(productStore, environmentStore, lockStore, deploymentStore, gitopsApplier, tagConventionDefault),
		router.RegisterStatusRoutes(productStore, environmentStore, statusReader),
		router.RegisterHistoryRoutes(productStore, deploymentStore),
		router.RegisterAdminRoutes(productStore, deploymentStore),
		router.RegisterStatsRoutes(statsStore),
	)
	registerSPA(r)

	// Sweep in_progress deployments older than the stale timeout once per minute.
	staleDur := handlers.StaleDeploymentTimeout()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := deploymentStore.MarkStaleInProgress(context.Background(), staleDur); err != nil {
				log.Printf("stale sweep: mark stale in_progress: %v", err)
			}
		}
	}()

	addr := ":" + port
	log.Printf("Server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func resolveIssuerURL() (string, error) {
	if url := os.Getenv("OIDC_ISSUER_URL"); url != "" {
		return url, nil
	}
	kcURL := strings.TrimRight(os.Getenv("KEYCLOAK_URL"), "/")
	kcRealm := strings.Trim(os.Getenv("KEYCLOAK_REALM"), "/")
	if kcURL == "" || kcRealm == "" {
		return "", fmt.Errorf("set OIDC_ISSUER_URL, or both KEYCLOAK_URL and KEYCLOAK_REALM")
	}
	return kcURL + "/realms/" + kcRealm, nil
}

// initGCRLister creates a GCR tag lister, falling back to a disabled stub on error.
// The returned closer is always non-nil; it is a no-op when the fallback is used.
func initGCRLister() (gcr.Lister, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := gcr.NewClient(ctx)
	if err != nil {
		log.Printf("Artifact Registry client unavailable (tag listing disabled): %v", err)
		return gcr.Disabled(), func() { /* no-op: disabled lister has no connection to close */ }
	}
	return client, func() {
		if err := client.Close(); err != nil {
			log.Printf("gcr client close: %v", err)
		}
	}
}

// GITOPS_REPO_URL is optional; all deploy, workload, and status requests fail with a clear error when absent.
func initGitOps() (handlers.GitOpsApplier, gitops.WorkloadReader, gitops.StatusReader) {
	w, err := gitops.NewWriterFromEnv()
	if err != nil {
		log.Printf("GitOps writer unavailable (deployments, workload discovery, and status disabled): %v", err)
		return &disabledGitOpsApplier{}, &disabledWorkloadReader{}, &disabledStatusReader{}
	}
	return w, w, w
}

type disabledGitOpsApplier struct{}

func (d *disabledGitOpsApplier) Apply(_ context.Context, _ gitops.ApplyParams) (string, error) {
	return "", fmt.Errorf("%w: set GITOPS_REPO_URL to enable deployments", gitops.ErrGitOpsNotConfigured)
}

type disabledWorkloadReader struct{}

func (d *disabledWorkloadReader) ListWorkloads(_ context.Context, _, _ string) ([]domain.Workload, error) {
	return nil, fmt.Errorf("%w: set GITOPS_REPO_URL to enable workload discovery", gitops.ErrGitOpsNotConfigured)
}

type disabledStatusReader struct{}

func (d *disabledStatusReader) ReadCurrentTags(_ context.Context, _, _ string) (map[string]string, error) {
	return nil, fmt.Errorf("%w: set GITOPS_REPO_URL to enable deployment status", gitops.ErrGitOpsNotConfigured)
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
