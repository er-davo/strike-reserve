//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"booking-service/internal/models"
	"booking-service/internal/repository"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlotRepository_FullCycle(t *testing.T) {
	ctx := context.Background()
	trxMngr := manager.Must(trmpgx.NewDefaultFactory(db))

	laneRepo := repository.NewLaneRepository(db, trmpgx.DefaultCtxGetter)
	slotRepo := repository.NewSlotRepository(db, trmpgx.DefaultCtxGetter)
	userRepo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter)
	bookingRepo := repository.NewBookingRepository(db, trmpgx.DefaultCtxGetter)

	err := trxMngr.Do(ctx, func(ctx context.Context) error {
		user := &models.User{ID: uuid.New(), Email: "slot_tester@test.com", Role: models.RoleUser}
		require.NoError(t, userRepo.GetOrCreateForDummy(ctx, user))

		lane := &models.Lane{
			ID:   uuid.New(),
			Name: "Slot Lab",
			Type: models.LaneTypeStandard,
		}
		require.NoError(t, laneRepo.Create(ctx, lane))

		testDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

		slot1ID := uuid.New()
		slot2ID := uuid.New()
		slots := []models.Slot{
			{ID: slot1ID, LaneID: lane.ID, StartTime: testDate.Add(10 * time.Hour), EndTime: testDate.Add(10*time.Hour + 30*time.Minute)},
			{ID: slot2ID, LaneID: lane.ID, StartTime: testDate.Add(11 * time.Hour), EndTime: testDate.Add(11*time.Hour + 30*time.Minute)},
		}
		require.NoError(t, slotRepo.CreateSlots(ctx, slots))

		t.Run("Get by id", func(t *testing.T) {
			s, err := slotRepo.GetByID(ctx, slot1ID)
			require.NoError(t, err)
			require.Equal(t, lane.ID, s.LaneID)

			_, err = slotRepo.GetByID(ctx, uuid.New())
			require.ErrorIs(t, err, repository.ErrNotFound)
		})

		t.Run("Check Initially Available", func(t *testing.T) {
			available, err := slotRepo.GetAvailableByLaneAndDate(ctx, lane.ID, testDate)
			require.NoError(t, err)
			assert.Len(t, available, 2)
		})

		t.Run("GetByRoomID - All Slots For Lane", func(t *testing.T) {
			foundSlots, err := slotRepo.GetByLaneID(ctx, lane.ID)
			require.NoError(t, err)
			assert.Len(t, foundSlots, 2, "Must return all slots for lane")
		})

		var b *models.Booking

		t.Run("Exclude Booked via BookingRepo", func(t *testing.T) {
			b = &models.Booking{
				ID:     uuid.New(),
				UserID: user.ID,
				SlotID: slot1ID,
				Status: models.StatusActive,
			}
			err := bookingRepo.Create(ctx, b)
			require.NoError(t, err)

			available, err := slotRepo.GetAvailableByLaneAndDate(ctx, lane.ID, testDate)
			require.NoError(t, err)
			assert.Len(t, available, 1)
		})

		t.Run("Restore Slot After Cancel", func(t *testing.T) {
			err := bookingRepo.Cancel(ctx, b.ID, user.ID)

			require.NoError(t, err)

			available, err := slotRepo.GetAvailableByLaneAndDate(ctx, lane.ID, testDate)
			require.NoError(t, err)
			assert.Len(t, available, 2, "Slot should be available after cancel")
		})

		return fmt.Errorf("rollback")
	})
	assert.Contains(t, err.Error(), "rollback")
}
