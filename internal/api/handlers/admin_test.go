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

func setupAdminRouter(ms *mockAdminStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewAdminHandlers(ms)
	r.GET("/api/v1/admin/products", h.GetAdminProducts)
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
				ID:        "id-alpha",
				Name:      "Alpha",
				Slug:      "alpha",
				CreatedAt: now(),
			},
			EnvironmentCount: 3,
			LastDeployedAt:   &deployedAt,
		},
		{
			Product: domain.Product{
				ID:        "id-beta",
				Name:      "Beta",
				Slug:      "beta",
				CreatedAt: now(),
			},
			EnvironmentCount: 0,
			LastDeployedAt:   nil,
		},
	}

	ms := &mockAdminStore{listWithStatsResult: stats}
	w := doPlain(setupAdminRouter(ms), http.MethodGet, "/api/v1/admin/products")
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
	w := doPlain(setupAdminRouter(ms), http.MethodGet, "/api/v1/admin/products")
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
	w := doPlain(setupAdminRouter(ms), http.MethodGet, "/api/v1/admin/products")
	assertStatus(t, w, http.StatusInternalServerError)
}
