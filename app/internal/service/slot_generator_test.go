package service_test

import (
	"context"
	"testing"
	"time"

	"booking-service/internal/mocks"
	"booking-service/internal/models"
	"booking-service/internal/service"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

func TestSlotGenerator_GenerateAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLaneRepo := mocks.NewMockLaneRepository(ctrl)
	mockSchRepo := mocks.NewMockScheduleRepository(ctrl)
	mockSlotRepo := mocks.NewMockSlotRepository(ctrl)

	slotDuration := 30 * time.Minute
	gen := service.NewSlotGenerator(
		mockLaneRepo,
		mockSchRepo,
		mockSlotRepo,
		slotDuration,
		1*time.Hour,
		14,
	)

	roomID := uuid.New()
	rooms := []models.Lane{{ID: roomID, Name: "Test Room"}}

	sch := &models.Schedule{
		LaneID:     roomID,
		StartTime:  models.HMSTime("09:00:00"),
		EndTime:    models.HMSTime("10:00:00"),
		DaysOfWeek: []int{1, 2, 3, 4, 5},
	}

	t.Run("Success Full Cycle", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockLaneRepo.EXPECT().GetAll(gomock.Any()).Return(rooms, nil)

		mockSchRepo.EXPECT().GetByLaneID(gomock.Any(), roomID).Return(sch, nil)

		mockSlotRepo.EXPECT().CreateSlots(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, slots []models.Slot) error {
				assert.NotEmpty(t, slots)
				for _, s := range slots {
					assert.Equal(t, roomID, s.LaneID)
					assert.Equal(t, slotDuration, s.EndTime.Sub(s.StartTime))
				}
				return nil
			},
		).AnyTimes()
		gen.GenerateAll(ctx)
	})

	t.Run("No Schedule - No Slots", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockLaneRepo.EXPECT().GetAll(gomock.Any()).Return(rooms, nil)
		mockSchRepo.EXPECT().GetByLaneID(gomock.Any(), roomID).Return(nil, nil)

		mockSlotRepo.EXPECT().CreateSlots(gomock.Any(), gomock.Any()).Times(0)

		gen.GenerateAll(ctx)
	})
}
