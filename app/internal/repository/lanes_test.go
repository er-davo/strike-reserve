//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"booking-service/internal/models"
	"booking-service/internal/repository"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaneRepository_FullCycle(t *testing.T) {
	ctx := context.Background()
	trxMngr := manager.Must(trmpgx.NewDefaultFactory(db))
	repo := repository.NewLaneRepository(db, trmpgx.DefaultCtxGetter)

	lane1 := &models.Lane{
		Name:        "Alpha",
		Description: ptr("Main lane"),
		Type:        models.LaneTypeStandard,
	}
	lane2 := &models.Lane{
		Name: "Beta",
		Type: models.LaneTypeStandard,
	}

	err := trxMngr.Do(ctx, func(ctx context.Context) error {
		t.Run("Create Lanes", func(t *testing.T) {
			err := repo.Create(ctx, lane1)
			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, lane1.ID)

			err = repo.Create(ctx, lane2)
			require.NoError(t, err)
		})

		t.Run("Get Lane By ID", func(t *testing.T) {
			found, err := repo.Get(ctx, lane1.ID)
			require.NoError(t, err)
			assert.Equal(t, lane1.Name, found.Name)
			assert.Equal(t, *lane1.Description, *found.Description)

			found2, err := repo.Get(ctx, lane2.ID)
			require.NoError(t, err)
			assert.Nil(t, found2.Description, "Description should be nil")
		})

		t.Run("Get All Lanes and Order", func(t *testing.T) {
			rooms, err := repo.GetAll(ctx)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, len(rooms), 2)

			var foundAlpha, foundBeta bool
			var alphaIdx, betaIdx int

			for i, r := range rooms {
				if r.ID == lane1.ID {
					foundAlpha = true
					alphaIdx = i
				}
				if r.ID == lane2.ID {
					foundBeta = true
					betaIdx = i
				}
			}

			assert.True(t, foundAlpha && foundBeta)
			assert.True(t, alphaIdx < betaIdx, "Room Alpha should be before Room Beta after sorting")
		})

		t.Run("Get By Non-Existent ID", func(t *testing.T) {
			randomID := uuid.New()
			found, err := repo.Get(ctx, randomID)
			assert.Error(t, err)
			assert.Nil(t, found)
		})

		return fmt.Errorf("rollback_rooms")
	})

	assert.Contains(t, err.Error(), "rollback_rooms")
}
