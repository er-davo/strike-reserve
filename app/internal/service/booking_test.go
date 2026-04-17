package service_test

import (
	"context"
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

func TestBookingService_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBookingRepo := mocks.NewMockBookingRepository(ctrl)
	mockSlotRepo := mocks.NewMockSlotRepository(ctrl)
	mockSlotCacheInvalidator := mocks.NewMockSlotCahceInvalidator(ctrl)

	txManager := service.TxManagerStub{}

	svc := service.NewBookingService(
		mockBookingRepo,
		mockSlotRepo,
		&txManager,
		mockSlotCacheInvalidator,
		time.Millisecond*100,
	)

	uID := uuid.New()
	sID := uuid.New()
	rID := uuid.New()
	now := time.Now()

	tests := []struct {
		name    string
		setup   func()
		wantErr error
	}{
		{
			name: "Success_WithMeeting",
			setup: func() {
				mockSlotRepo.EXPECT().GetByID(gomock.Any(), sID).Return(&models.Slot{
					ID:        sID,
					LaneID:    rID,
					StartTime: now,
				}, nil)
				mockBookingRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				mockSlotCacheInvalidator.EXPECT().InvalidateSlotsCache(
					gomock.Any(),
					rID,
					service.SafeTruncate(now),
				).Return()
			},
			wantErr: nil,
		},
		{
			name: "Err_SlotAlreadyUsed_Conflict",
			setup: func() {
				mockSlotRepo.EXPECT().GetByID(gomock.Any(), sID).Return(&models.Slot{ID: sID}, nil)
				mockBookingRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrConflict)
			},
			wantErr: service.ErrSlotIsUsed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

			tt.setup()
			res, err := svc.Create(ctx, uID, sID)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr.Error())
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
			}
		})
	}
}

func TestBookingService_Cancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockBookingRepository(ctrl)
	mockSlotRepo := mocks.NewMockSlotRepository(ctrl)
	mockSlotCacheInvalidator := mocks.NewMockSlotCahceInvalidator(ctrl)

	svc := service.NewBookingService(
		mockRepo,
		mockSlotRepo,
		&service.TxManagerStub{},
		mockSlotCacheInvalidator,
		0,
	)

	bID := uuid.New()
	uID := uuid.New()
	rID := uuid.New()
	sID := uuid.New()
	startTime := time.Now()

	t.Run("Success_CancelActive", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockRepo.EXPECT().Get(gomock.Any(), bID).Return(&models.Booking{
			UserID: uID,
			SlotID: sID,
			Status: models.StatusActive,
		}, nil)
		mockSlotRepo.EXPECT().GetByID(gomock.Any(), sID).Return(&models.Slot{
			ID:        sID,
			LaneID:    rID,
			StartTime: startTime,
		}, nil)
		mockRepo.EXPECT().Cancel(gomock.Any(), bID, uID).Return(nil)
		mockSlotCacheInvalidator.EXPECT().InvalidateSlotsCache(gomock.Any(), rID, service.SafeTruncate(startTime)).Return()

		res, err := svc.Cancel(ctx, bID, uID)
		assert.NoError(t, err)
		assert.Equal(t, models.StatusCancelled, res.Status)
	})

	t.Run("Err_Forbidden_NotOwner", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockRepo.EXPECT().Get(gomock.Any(), bID).Return(&models.Booking{UserID: uuid.New(), Status: models.StatusActive}, nil)

		res, err := svc.Cancel(ctx, bID, uID)
		assert.ErrorIs(t, err, service.ErrForbidden)
		assert.Nil(t, res)
	})
}

func TestBookingService_GetList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockBookingRepository(ctrl)
	svc := service.NewBookingService(mockRepo, nil, &service.TxManagerStub{}, nil, 0)

	t.Run("Success", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		expected := &models.BookingList{Total: 1}
		mockRepo.EXPECT().GetList(gomock.Any(), uint64(10), uint64(1)).Return(expected, nil)

		res, err := svc.GetList(ctx, 10, 1)
		assert.NoError(t, err)
		assert.Equal(t, expected, res)
	})
}
