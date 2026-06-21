package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/handlers"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deployStrPtr(s string) *string { return &s }

var fixtureDeployment = &domain.Deployment{
	ID:               "depl-uuid-1",
	ActorDisplayName: "sub-user-1",
	ProductID:        "prod-id-1",
	EnvironmentID:    "env-id-1",
	ComponentName:    "api",
	EnvironmentName:  "dev",
	Tag:              "v1.2.3",
	DeployedAt:       time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
	CommitSHA:        deployStrPtr("abc123def456"),
	Outcome:          domain.OutcomeSuccess,
}

func newGetDeploymentRouter(
	ps store.ProductStore,
	ds store.DeploymentStore,
	identity *domain.UserIdentity,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewDeploymentHandlers(ps, nil, nil, ds, successApplier(), "")
	injectIdentity := func(c *gin.Context) {
		if identity != nil {
			c.Set(domain.IdentityContextKey, identity)
		}
		c.Next()
	}
	api := r.Group("/api/v1", injectIdentity)
	api.GET("/deployments/:deploymentID", h.GetDeployment)
	return r
}

func doGetDeployment(t *testing.T, r *gin.Engine, deploymentID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/"+deploymentID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetDeployment_Success(t *testing.T) {
	ps := &mockProductStore{
		getByIDFn: func(_ context.Context, id string) (*domain.Product, error) {
			if id == "prod-id-1" {
				return &domain.Product{ID: "prod-id-1", Slug: "my-service"}, nil
			}
			return nil, store.ErrNotFound
		},
	}
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, id string) (*domain.Deployment, error) {
			if id == fixtureDeployment.ID {
				return fixtureDeployment, nil
			}
			return nil, store.ErrDeploymentNotFound
		},
	}
	identity := &domain.UserIdentity{
		Sub:          "user-1",
		ProductRoles: map[string]domain.Role{"my-service": domain.RoleViewer},
	}

	r := newGetDeploymentRouter(ps, ds, identity)
	w := doGetDeployment(t, r, fixtureDeployment.ID)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, fixtureDeployment.ID, resp["id"])
	assert.Equal(t, *fixtureDeployment.CommitSHA, resp["commit_sha"])
	assert.Equal(t, domain.OutcomeSuccess, resp["outcome"])
	assert.Equal(t, fixtureDeployment.Tag, resp["tag"])
	assert.Equal(t, fixtureDeployment.ComponentName, resp["component_name"])
	assert.Equal(t, "2026-06-15T10:00:00Z", resp["deployed_at"])
}

func TestGetDeployment_NotFound_Returns404(t *testing.T) {
	ps := &mockProductStore{}
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Deployment, error) {
			return nil, store.ErrDeploymentNotFound
		},
	}
	identity := &domain.UserIdentity{Sub: "user-1", IsDevOpsAdmin: true}

	r := newGetDeploymentRouter(ps, ds, identity)
	w := doGetDeployment(t, r, "nonexistent-uuid")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetDeployment_NoProductAccess_Returns404(t *testing.T) {
	ps := &mockProductStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return &domain.Product{ID: "prod-id-1", Slug: "other-product"}, nil
		},
	}
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Deployment, error) {
			return fixtureDeployment, nil
		},
	}
	identity := &domain.UserIdentity{
		Sub:          "user-1",
		ProductRoles: map[string]domain.Role{"some-other-product": domain.RoleViewer},
	}

	r := newGetDeploymentRouter(ps, ds, identity)
	w := doGetDeployment(t, r, fixtureDeployment.ID)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetDeployment_AdminBypassesProductCheck(t *testing.T) {
	ps := &mockProductStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return &domain.Product{ID: "prod-id-1", Slug: "any-product"}, nil
		},
	}
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Deployment, error) {
			return fixtureDeployment, nil
		},
	}
	identity := &domain.UserIdentity{Sub: "admin", IsDevOpsAdmin: true}

	r := newGetDeploymentRouter(ps, ds, identity)
	w := doGetDeployment(t, r, fixtureDeployment.ID)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetDeployment_StoreError_Returns500(t *testing.T) {
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Deployment, error) {
			return nil, errors.New("db connection reset")
		},
	}
	identity := &domain.UserIdentity{Sub: "user-1", IsDevOpsAdmin: true}

	r := newGetDeploymentRouter(&mockProductStore{}, ds, identity)
	w := doGetDeployment(t, r, fixtureDeployment.ID)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetDeployment_ProductStoreError_Returns500(t *testing.T) {
	ps := &mockProductStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return nil, errors.New("db timeout")
		},
	}
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Deployment, error) {
			return fixtureDeployment, nil
		},
	}
	identity := &domain.UserIdentity{Sub: "user-1", IsDevOpsAdmin: true}

	r := newGetDeploymentRouter(ps, ds, identity)
	w := doGetDeployment(t, r, fixtureDeployment.ID)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetDeployment_NoIdentity_Returns401(t *testing.T) {
	ds := &mockDeploymentStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Deployment, error) {
			return fixtureDeployment, nil
		},
	}
	ps := &mockProductStore{
		getByIDFn: func(_ context.Context, _ string) (*domain.Product, error) {
			return &domain.Product{ID: "prod-id-1", Slug: "my-service"}, nil
		},
	}

	r := newGetDeploymentRouter(ps, ds, nil)
	w := doGetDeployment(t, r, fixtureDeployment.ID)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
