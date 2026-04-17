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

func TestBookingRepository_FullCycle(t *testing.T) {
	ctx := context.Background()
	trxMngr := manager.Must(trmpgx.NewDefaultFactory(db))

	laneRepo := repository.NewLaneRepository(db, trmpgx.DefaultCtxGetter)
	slotRepo := repository.NewSlotRepository(db, trmpgx.DefaultCtxGetter)
	userRepo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter)
	bookingRepo := repository.NewBookingRepository(db, trmpgx.DefaultCtxGetter)

	t.Run("Positive Scenarios", func(t *testing.T) {
		_ = trxMngr.Do(ctx, func(ctx context.Context) error {
			user := &models.User{ID: uuid.New(), Email: "booker@test.com", Role: models.RoleUser}
			require.NoError(t, userRepo.GetOrCreateForDummy(ctx, user))

			lane := &models.Lane{
				ID:   uuid.New(),
				Name: "Positive Lane",
				Type: models.LaneTypeStandard,
			}
			require.NoError(t, laneRepo.Create(ctx, lane))

			futureDate1 := time.Now().Add(24 * time.Hour).Truncate(time.Second)
			targetSlot := models.Slot{ID: uuid.New(), LaneID: lane.ID, StartTime: futureDate1, EndTime: futureDate1.Add(30 * time.Minute)}
			require.NoError(t, slotRepo.CreateSlots(ctx, []models.Slot{targetSlot}))

			booking := &models.Booking{
				UserID: user.ID,
				SlotID: targetSlot.ID,
				Status: models.StatusActive,
			}

			t.Run("Create, Get and Cancel Lifecycle", func(t *testing.T) {
				err := bookingRepo.Create(ctx, booking)
				require.NoError(t, err)

				b, err := bookingRepo.Get(ctx, booking.ID)
				assert.NoError(t, err)
				assert.Equal(t, booking, b)

				err = bookingRepo.Cancel(ctx, booking.ID, user.ID)
				assert.NoError(t, err)

				err = bookingRepo.Cancel(ctx, booking.ID, user.ID)
				assert.NoError(t, err, "Repeat cancellation must be idempotent (return nil)")
			})

			t.Run("Get - not found", func(t *testing.T) {
				id := uuid.New()

				_, err := bookingRepo.Get(ctx, id)
				assert.ErrorIs(t, err, repository.ErrNotFound)
			})

			return fmt.Errorf("rollback_positive")
		})
	})

	t.Run("Pagination and Filtering", func(t *testing.T) {
		_ = trxMngr.Do(ctx, func(ctx context.Context) error {
			user := &models.User{ID: uuid.New(), Email: "page@test.com", Role: models.RoleUser}
			require.NoError(t, userRepo.GetOrCreateForDummy(ctx, user))
			lane := &models.Lane{
				ID:   uuid.New(),
				Name: "Page Lane",
				Type: models.LaneTypeStandard,
			}
			require.NoError(t, laneRepo.Create(ctx, lane))

			now := time.Now().UTC().Truncate(time.Second)

			s1Time := now.Add(-1 * time.Hour)
			s2Time := now.Add(1 * time.Hour)
			s3Time := now.Add(2 * time.Hour)

			s1 := models.Slot{ID: uuid.New(), LaneID: lane.ID, StartTime: s1Time, EndTime: s1Time.Add(30 * time.Minute)}
			s2 := models.Slot{ID: uuid.New(), LaneID: lane.ID, StartTime: s2Time, EndTime: s2Time.Add(30 * time.Minute)}
			s3 := models.Slot{ID: uuid.New(), LaneID: lane.ID, StartTime: s3Time, EndTime: s3Time.Add(30 * time.Minute)}
			require.NoError(t, slotRepo.CreateSlots(ctx, []models.Slot{s1, s2, s3}))

			require.NoError(t, bookingRepo.Create(ctx, &models.Booking{UserID: user.ID, SlotID: s1.ID, Status: models.StatusActive}))
			require.NoError(t, bookingRepo.Create(ctx, &models.Booking{UserID: user.ID, SlotID: s2.ID, Status: models.StatusActive}))
			require.NoError(t, bookingRepo.Create(ctx, &models.Booking{UserID: user.ID, SlotID: s3.ID, Status: models.StatusActive}))

			t.Run("GetList returns total count and respects limit", func(t *testing.T) {
				limit := uint64(2)
				page := uint64(0)

				list, err := bookingRepo.GetList(ctx, limit, page)
				require.NoError(t, err)

				assert.Equal(t, uint64(3), list.Total, "Total count should be 3 regardless of limit")
				assert.Len(t, list.Bookings, 2, "Should return only 2 items due to limit")

				listPage2, err := bookingRepo.GetList(ctx, limit, 1)
				require.NoError(t, err)
				assert.Len(t, listPage2.Bookings, 1, "Should return the remaining 1 item on the second page")
			})

			t.Run("GetActiveByUserID filters past slots", func(t *testing.T) {
				active, err := bookingRepo.GetActiveByUserID(ctx, user.ID)
				require.NoError(t, err)
				assert.Len(t, active, 2)
			})

			return fmt.Errorf("rollback_paging")
		})
	})
}
