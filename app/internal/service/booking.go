package service

import (
	"context"
	"errors"
	"time"

	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type BookingService struct {
	bookingRepo BookingRepository
	slotRepo    SlotRepository

	trManager TxManager

	slotCacheInvalidator SlotCahceInvalidator

	conSrvTimeout time.Duration
}

func NewBookingService(
	b BookingRepository,
	s SlotRepository,
	trm TxManager,
	slotCacheInvalidator SlotCahceInvalidator,
	conSrvTimeout time.Duration,
) *BookingService {
	return &BookingService{
		bookingRepo:          b,
		slotRepo:             s,
		trManager:            trm,
		slotCacheInvalidator: slotCacheInvalidator,
		conSrvTimeout:        conSrvTimeout,
	}
}

func (s *BookingService) Create(ctx context.Context, userID, slotID uuid.UUID) (*models.Booking, error) {
	l := logger.FromContext(ctx)
	l.Info("attempting to create booking",
		zap.String("user_id", userID.String()),
		zap.String("slot_id", slotID.String()),
	)

	var (
		booking *models.Booking
		slot    *models.Slot
	)

	err := s.trManager.Do(ctx, func(ctx context.Context) error {
		txLog := logger.FromContext(ctx)
		var err error

		slot, err = s.slotRepo.GetByID(ctx, slotID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				txLog.Error("slot not existing", zap.String("slot_id", slotID.String()), zap.Error(err))
				return ErrSlotNotExisting
			}
			txLog.Error("failed to get slot", zap.String("slot_id", slotID.String()), zap.Error(err))
			return err
		}

		booking = &models.Booking{
			ID:     uuid.New(),
			UserID: userID,
			SlotID: slotID,
			Status: models.StatusActive,
		}

		if err := s.bookingRepo.Create(ctx, booking); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				txLog.Error("slot is used", zap.String("slot_id", slotID.String()), zap.Error(err))
				return ErrSlotIsUsed
			}
			txLog.Error("failed to create booking", zap.Error(err))
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.slotCacheInvalidator.InvalidateSlotsCache(ctx, slot.LaneID, SafeTruncate(slot.StartTime))

	l.Info("booking created successfully", zap.String("id", booking.ID.String()))

	return booking, nil
}

func (s *BookingService) GetList(ctx context.Context, pageSize, page uint64) (*models.BookingList, error) {
	l := logger.FromContext(ctx)
	l.Debug("fetching bookings list", zap.Uint64("page_size", pageSize), zap.Uint64("page", page))

	list, err := s.bookingRepo.GetList(ctx, pageSize, page)
	if err != nil {
		l.Error("failed to fetch bookings list", zap.Error(err))
		return nil, err
	}
	return list, nil
}

func (s *BookingService) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Booking, error) {
	l := logger.FromContext(ctx)
	l.Debug("fetching user bookings", zap.String("user_id", userID.String()))

	bookings, err := s.bookingRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		l.Error("failed to fetch user bookings", zap.Error(err), zap.String("user_id", userID.String()))
		return nil, err
	}
	return bookings, nil
}

func (s *BookingService) Cancel(ctx context.Context, bookingID, userID uuid.UUID) (*models.Booking, error) {
	l := logger.FromContext(ctx)
	l.Info("attempting to cancel booking",
		zap.String("booking_id", bookingID.String()),
		zap.String("user_id", userID.String()),
	)

	var (
		booking    *models.Booking
		slot       *models.Slot
		wasChanged bool
	)
	err := s.trManager.Do(ctx, func(ctx context.Context) error {
		txLog := logger.FromContext(ctx)
		var err error

		booking, err = s.bookingRepo.Get(ctx, bookingID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				txLog.Warn("cancel failed: booking not found", zap.String("booking_id", bookingID.String()))
				return ErrNotFound
			}
			return err
		}

		if booking.UserID != userID {
			txLog.Warn("cancel forbidden: user is not the owner",
				zap.String("booking_id", bookingID.String()),
				zap.String("request_user_id", userID.String()),
				zap.String("owner_user_id", booking.UserID.String()),
			)
			return ErrForbidden
		}

		if booking.Status == models.StatusCancelled {
			txLog.Debug("booking already cancelled", zap.String("booking_id", bookingID.String()))
			wasChanged = true
			return nil
		}

		slot, err = s.slotRepo.GetByID(ctx, booking.SlotID)
		if err != nil {
			txLog.Error("failed to get slot by id", zap.Error(err), zap.String("slot_id", booking.SlotID.String()))
			return err
		}

		err = s.bookingRepo.Cancel(ctx, bookingID, userID)
		if err != nil {
			txLog.Error("repository failed to cancel booking", zap.Error(err), zap.String("booking_id", bookingID.String()))
			return err
		}

		booking.Status = models.StatusCancelled
		return nil
	})

	if err != nil {
		l.Error(
			"error on canceling booking",
			zap.Error(err),
			zap.String("booking_id", bookingID.String()),
		)
		return nil, err
	}

	if !wasChanged {
		s.slotCacheInvalidator.InvalidateSlotsCache(ctx, slot.LaneID, SafeTruncate(slot.StartTime))
	}

	l.Info("booking cancelled successfully", zap.String("booking_id", bookingID.String()))

	return booking, nil
}
