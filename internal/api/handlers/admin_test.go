package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

// mockAdminStore embeds mockProductStore (defined in products_test.go) and overrides
// ListWithStats so tests can control its behaviour.
type mockAdminStore struct {
	mockProductStore
	listWithStatsResult []domain.ProductStats
	listWithStatsErr    error
}

func (m *mockAdminStore) ListWithStats(_ context.Context) ([]domain.ProductStats, error) {
	return m.listWithStatsResult, m.listWithStatsErr
}

// mockActivityStore is a minimal DeploymentStore implementation for GetActivity tests.
type mockActivityStore struct {
	listActivityResult []domain.Deployment
	listActivityErr    error
	markStaleErr       error
}

func (m *mockActivityStore) ListActivity(_ context.Context, _ int) ([]domain.Deployment, error) {
	return m.listActivityResult, m.listActivityErr
}
func (m *mockActivityStore) MarkStaleInProgress(_ context.Context, _ time.Duration) error {
	return m.markStaleErr
}
func (m *mockActivityStore) Create(_ context.Context, _ *domain.Deployment) error { return nil }
func (m *mockActivityStore) GetByID(_ context.Context, _ string) (*domain.Deployment, error) {
	return nil, nil
}
func (m *mockActivityStore) ListByProduct(_ context.Context, _ string, _, _ int) ([]domain.Deployment, int, error) {
	return nil, 0, nil
}
func (m *mockActivityStore) ListAll(_ context.Context, _, _ int) ([]domain.Deployment, int, error) {
	return nil, 0, nil
}
func (m *mockActivityStore) UpdateOutcome(_ context.Context, _, _ string, _ *string, _ *string) error {
	return nil
}
func (m *mockActivityStore) Delete(_ context.Context, _ string) error { return nil }

var _ store.DeploymentStore = (*mockActivityStore)(nil)

func setupAdminRouter(ms *mockAdminStore, ds store.DeploymentStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewAdminHandlers(ms, ds)
	r.GET("/api/v1/admin/products", h.GetAdminProducts)
	r.GET("/api/v1/admin/activity", h.GetActivity)
	return r
}

// TestGetAdminProducts_ReturnsStats verifies that two products — one with
// environments and a last_deployed_at value, one without — are returned
// correctly with all expected fields.
func TestGetAdminProducts_ReturnsStats(t *testing.T) {
	deployedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	stats := []domain.ProductStats{
		{
			Product: domain.Product{
				ID:             "id-alpha",
				Name:           "Alpha",
				Slug:           "alpha",
				CreatedAt:      now(),
				LastDeployedAt: &deployedAt,
			},
			EnvironmentCount: 3,
		},
		{
			Product: domain.Product{
				ID:        "id-beta",
				Name:      "Beta",
				Slug:      "beta",
				CreatedAt: now(),
			},
			EnvironmentCount: 0,
		},
	}

	ms := &mockAdminStore{listWithStatsResult: stats}
	w := doPlain(setupAdminRouter(ms, &mockActivityStore{}), http.MethodGet, "/api/v1/admin/products")
	assertStatus(t, w, http.StatusOK)

	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 products, got %d", len(resp))
	}

	// Verify first product (with environments and last_deployed_at).
	first := resp[0]
	if first["id"] != "id-alpha" {
		t.Errorf("expected id id-alpha, got %v", first["id"])
	}
	if first["slug"] != "alpha" {
		t.Errorf("expected slug alpha, got %v", first["slug"])
	}
	if envCount, ok := first["environment_count"].(float64); !ok || int(envCount) != 3 {
		t.Errorf("expected environment_count 3, got %v", first["environment_count"])
	}
	if first["last_deployed_at"] == nil {
		t.Error("expected last_deployed_at to be present for first product")
	}

	// Verify second product (no environments, no last_deployed_at).
	second := resp[1]
	if second["id"] != "id-beta" {
		t.Errorf("expected id id-beta, got %v", second["id"])
	}
	if envCount, ok := second["environment_count"].(float64); !ok || int(envCount) != 0 {
		t.Errorf("expected environment_count 0, got %v", second["environment_count"])
	}
	if second["last_deployed_at"] != nil {
		t.Errorf("expected last_deployed_at to be null for second product, got %v", second["last_deployed_at"])
	}
}

// TestGetAdminProducts_EmptyList verifies that an empty store returns a JSON
// array (not null) with status 200.
func TestGetAdminProducts_EmptyList(t *testing.T) {
	ms := &mockAdminStore{listWithStatsResult: []domain.ProductStats{}}
	w := doPlain(setupAdminRouter(ms, &mockActivityStore{}), http.MethodGet, "/api/v1/admin/products")
	assertStatus(t, w, http.StatusOK)

	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

// TestGetAdminProducts_StoreError verifies that a store error produces a 500.
func TestGetAdminProducts_StoreError(t *testing.T) {
	ms := &mockAdminStore{listWithStatsErr: errors.New("db is down")}
	w := doPlain(setupAdminRouter(ms, &mockActivityStore{}), http.MethodGet, "/api/v1/admin/products")
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestGetActivity_ReturnsActivityList(t *testing.T) {
	deployedAt1 := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	deployedAt2 := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	sha := "abc123"
	deployments := []domain.Deployment{
		{
			ID:               "dep-id-1",
			ProductSlug:      "my-service",
			ActorDisplayName: "Sara DevOps",
			Tag:              "v1.0.0",
			ComponentName:    "api",
			EnvironmentName:  "dev",
			DeployedAt:       deployedAt1,
			Outcome:          domain.OutcomeSuccess,
			CommitSHA:        &sha,
		},
		{
			ID:               "dep-id-2",
			ProductSlug:      "other-service",
			ActorDisplayName: "Marco Lead",
			Tag:              "v2.0.0",
			ComponentName:    "worker",
			EnvironmentName:  "production",
			DeployedAt:       deployedAt2,
			Outcome:          domain.OutcomeInProgress,
		},
	}
	ds := &mockActivityStore{listActivityResult: deployments}
	w := doPlain(setupAdminRouter(&mockAdminStore{}, ds), http.MethodGet, "/api/v1/admin/activity")
	assertStatus(t, w, http.StatusOK)

	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp))
	}
	if resp[0]["id"] != "dep-id-1" {
		t.Errorf("expected id dep-id-1, got %v", resp[0]["id"])
	}
	if resp[0]["product_slug"] != "my-service" {
		t.Errorf("expected product_slug my-service, got %v", resp[0]["product_slug"])
	}
	if resp[0]["outcome"] != domain.OutcomeSuccess {
		t.Errorf("expected outcome success, got %v", resp[0]["outcome"])
	}
	if resp[1]["outcome"] != domain.OutcomeInProgress {
		t.Errorf("expected outcome in_progress, got %v", resp[1]["outcome"])
	}
	if resp[0]["deployed_at"] != "2026-06-20T10:00:00Z" {
		t.Errorf("unexpected deployed_at: %v", resp[0]["deployed_at"])
	}
}

func TestGetActivity_EmptyList(t *testing.T) {
	ds := &mockActivityStore{listActivityResult: []domain.Deployment{}}
	w := doPlain(setupAdminRouter(&mockAdminStore{}, ds), http.MethodGet, "/api/v1/admin/activity")
	assertStatus(t, w, http.StatusOK)

	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestGetActivity_StoreError(t *testing.T) {
	ds := &mockActivityStore{listActivityErr: errors.New("db error")}
	w := doPlain(setupAdminRouter(&mockAdminStore{}, ds), http.MethodGet, "/api/v1/admin/activity")
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestGetActivity_MarkStaleError_StillReturnsActivity(t *testing.T) {
	// The stale sweep was removed from GetActivity; this test documents the contract
	// that a MarkStaleInProgress error (now handled in the background sweep) would
	// not affect the activity response. We simulate it by injecting a store that
	// returns an error from MarkStaleInProgress and verifying that GetActivity
	// still returns the activity list with 200.
	deployedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	ds := &mockActivityStore{
		listActivityResult: []domain.Deployment{
			{
				ID:          "dep-id-1",
				ProductSlug: "my-service",
				Outcome:     domain.OutcomeSuccess,
				DeployedAt:  deployedAt,
			},
		},
		markStaleErr: errors.New("db timeout"), // non-fatal, not called by GetActivity
	}
	w := doPlain(setupAdminRouter(&mockAdminStore{}, ds), http.MethodGet, "/api/v1/admin/activity")
	assertStatus(t, w, http.StatusOK)

	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 1 {
		t.Errorf("expected 1 item, got %d", len(resp))
	}
}
