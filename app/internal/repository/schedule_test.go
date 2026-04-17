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

func TestScheduleRepository_FullCycle(t *testing.T) {
	ctx := context.Background()
	trxMngr := manager.Must(trmpgx.NewDefaultFactory(db))

	laneRepo := repository.NewLaneRepository(db, trmpgx.DefaultCtxGetter)
	scheduleRepo := repository.NewScheduleRepository(db, trmpgx.DefaultCtxGetter)

	err := trxMngr.Do(ctx, func(ctx context.Context) error {
		room := &models.Lane{
			ID:   uuid.New(),
			Name: "Single Schedule Lane",
			Type: models.LaneTypeStandard,
		}
		err := laneRepo.Create(ctx, room)
		require.NoError(t, err)

		t.Run("Create Schedule Success", func(t *testing.T) {
			s := &models.Schedule{
				LaneID:     room.ID,
				DaysOfWeek: []int{1, 2, 3, 4, 5},
				StartTime:  "09:00:00",
				EndTime:    "18:00:00",
			}

			err := scheduleRepo.Create(ctx, s)
			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, s.ID)

			found, err := scheduleRepo.GetByLaneID(ctx, room.ID)
			require.NoError(t, err)
			require.NotNil(t, found)
			assert.Equal(t, s.ID, found.ID)
			assert.ElementsMatch(t, []int{1, 2, 3, 4, 5}, found.DaysOfWeek)
		})

		t.Run("DB Constraint: Only One Schedule Per Room", func(t *testing.T) {
			duplicateSch := &models.Schedule{
				LaneID:     room.ID,
				DaysOfWeek: []int{6, 7},
				StartTime:  "10:00:00",
				EndTime:    "15:00:00",
			}

			err := scheduleRepo.Create(ctx, duplicateSch)
			assert.Error(t, err, "Should fail because room already has a schedule")
		})

		t.Run("Get Non-Existent Schedule", func(t *testing.T) {
			nonExistentRoomID := uuid.New()
			found, err := scheduleRepo.GetByLaneID(ctx, nonExistentRoomID)

			assert.Error(t, err)
			assert.Nil(t, found)
		})

		return fmt.Errorf("rollback_schedules")
	})

	assert.Contains(t, err.Error(), "rollback_schedules")
}
