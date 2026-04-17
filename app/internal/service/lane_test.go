package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"booking-service/internal/mocks"
	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/internal/service"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

func TestRoomService_CreateRoom(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLaneRepo := mocks.NewMockLaneRepository(ctrl)
	svc := service.NewLaneService(
		mockLaneRepo,
		nil,
		nil,
		0,
		&service.TxManagerStub{},
		nil,
		14,
	)

	description := "Bowling lane a"
	laneCreate := models.LaneCreate{
		Name:        "Bowling lane A",
		Description: &description,
	}

	t.Run("Success", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		mockLaneRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

		res, err := svc.CreateLane(ctx, laneCreate)
		assert.NoError(t, err)
		assert.Equal(t, laneCreate.Name, res.Name)
		assert.NotEqual(t, uuid.Nil, res.ID)
	})

	t.Run("Repo Error", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		mockLaneRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

		res, err := svc.CreateLane(ctx, laneCreate)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestRoomService_GetAvailableSlots(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLaneRepo := mocks.NewMockLaneRepository(ctrl)
	mockSlotRepo := mocks.NewMockSlotRepository(ctrl)
	mockCache := mocks.NewMockCache[service.SlotCacheKey, []models.Slot](ctrl)

	svc := service.NewLaneService(
		mockLaneRepo,
		mockSlotRepo,
		nil,
		0,
		&service.TxManagerStub{},
		mockCache,
		14,
	)

	roomID := uuid.New()
	date := time.Now()
	cacheKey := service.SlotCacheKey{
		LaneID: roomID,
		Date:   date.Format("2006-01-02"),
	}

	t.Run("Success from Cache", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		expectedSlots := []models.Slot{{ID: uuid.New()}}

		mockCache.EXPECT().Get(gomock.Any(), cacheKey).Return(expectedSlots, true)

		slots, err := svc.GetAvailableSlots(ctx, roomID, date)

		assert.NoError(t, err)
		assert.Equal(t, expectedSlots, slots)
	})

	t.Run("Cache Miss - Success from Repo", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		expectedSlots := []models.Slot{{ID: uuid.New()}}

		mockCache.EXPECT().Get(gomock.Any(), cacheKey).Return(nil, false)
		mockLaneRepo.EXPECT().Get(gomock.Any(), roomID).Return(&models.Lane{ID: roomID}, nil)
		mockSlotRepo.EXPECT().GetAvailableByLaneAndDate(gomock.Any(), roomID, date).Return(expectedSlots, nil)
		mockCache.EXPECT().Set(gomock.Any(), cacheKey, expectedSlots)

		slots, err := svc.GetAvailableSlots(ctx, roomID, date)

		assert.NoError(t, err)
		assert.Len(t, slots, 1)
	})
}

func TestRoomService_CreateSchedule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLaneRepo := mocks.NewMockLaneRepository(ctrl)
	mockSchRepo := mocks.NewMockScheduleRepository(ctrl)
	mockSlotRepo := mocks.NewMockSlotRepository(ctrl)

	slotDuration := 30 * time.Minute
	svc := service.NewLaneService(
		mockLaneRepo,
		mockSlotRepo,
		mockSchRepo,
		slotDuration,
		&service.TxManagerStub{},
		nil,
		14,
	)

	roomID := uuid.New()
	startTime := models.HMSTime("09:00:00")
	endTime := models.HMSTime("11:00:00")

	sch := &models.Schedule{
		LaneID:     roomID,
		StartTime:  startTime,
		EndTime:    endTime,
		DaysOfWeek: []int{1, 2, 3, 4, 5},
	}

	t.Run("Success Generation", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockLaneRepo.EXPECT().Get(gomock.Any(), roomID).Return(&models.Lane{}, nil)
		mockSchRepo.EXPECT().GetByLaneID(gomock.Any(), roomID).Return(nil, nil)
		mockSchRepo.EXPECT().Create(gomock.Any(), sch).Return(nil)

		mockSlotRepo.EXPECT().CreateSlots(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, slots []models.Slot) error {
			assert.NotEmpty(t, slots)
			return nil
		})

		err := svc.CreateSchedule(ctx, sch)
		assert.NoError(t, err)
	})

	t.Run("Lane Not Found", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		mockLaneRepo.EXPECT().Get(gomock.Any(), roomID).Return(nil, repository.ErrNotFound)

		err := svc.CreateSchedule(ctx, sch)
		assert.ErrorIs(t, err, service.ErrLaneNotFound)
	})
}

func TestRoomService_InvalidateSlotsCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCache := mocks.NewMockCache[service.SlotCacheKey, []models.Slot](ctrl)
	svc := service.NewLaneService(nil, nil, nil, 0, nil, mockCache, 14)

	roomID := uuid.New()
	date := time.Now()
	expectedKey := service.SlotCacheKey{
		LaneID: roomID,
		Date:   date.Format("2006-01-02"),
	}

	t.Run("Success Delete", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		mockCache.EXPECT().Delete(gomock.Any(), expectedKey).Return()

		svc.InvalidateSlotsCache(ctx, roomID, date)
	})
}
