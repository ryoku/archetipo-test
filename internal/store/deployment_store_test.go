package store_test

import (
	"context"
	"testing"

	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentStore_CreateAndGetByID(t *testing.T) {
	pool := newTestPool(t)
	productID, envID := insertLockFixtures(t, pool)
	s := store.NewDeploymentStore(pool)

	d := &domain.Deployment{
		ActorSub:      "user-sub-123",
		ProductID:     productID,
		EnvironmentID: envID,
		Workload:      "api",
		Tag:           "v1.2.3",
		CommitSHA:     "abc123def456",
		Outcome:       domain.OutcomeSuccess,
		ErrorMessage:  "",
	}

	require.NoError(t, s.Create(context.Background(), d))
	assert.NotEmpty(t, d.ID)
	assert.False(t, d.DeployedAt.IsZero())

	got, err := s.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, d.ID, got.ID)
	assert.Equal(t, d.ActorSub, got.ActorSub)
	assert.Equal(t, d.ProductID, got.ProductID)
	assert.Equal(t, d.EnvironmentID, got.EnvironmentID)
	assert.Equal(t, d.Workload, got.Workload)
	assert.Equal(t, d.Tag, got.Tag)
	assert.Equal(t, d.CommitSHA, got.CommitSHA)
	assert.Equal(t, d.Outcome, got.Outcome)
	assert.Equal(t, d.ErrorMessage, got.ErrorMessage)
}

func TestDeploymentStore_CreateFailure_Outcome(t *testing.T) {
	pool := newTestPool(t)
	productID, envID := insertLockFixtures(t, pool)
	s := store.NewDeploymentStore(pool)

	d := &domain.Deployment{
		ActorSub:      "user-sub-456",
		ProductID:     productID,
		EnvironmentID: envID,
		Workload:      "worker",
		Tag:           "latest",
		CommitSHA:     "",
		Outcome:       domain.OutcomeFailure,
		ErrorMessage:  "push failed: permission denied",
	}

	require.NoError(t, s.Create(context.Background(), d))

	got, err := s.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeFailure, got.Outcome)
	assert.Equal(t, "push failed: permission denied", got.ErrorMessage)
}

func TestDeploymentStore_GetByID_NotFound(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewDeploymentStore(pool)

	_, err := s.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)
}

func TestDeploymentStore_GetByID_MalformedUUID_Returns404Sentinel(t *testing.T) {
	pool := newTestPool(t)
	s := store.NewDeploymentStore(pool)

	_, err := s.GetByID(context.Background(), "not-a-uuid")
	assert.ErrorIs(t, err, store.ErrDeploymentNotFound)
}
