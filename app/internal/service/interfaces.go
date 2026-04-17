//go:generate mockgen -source=interfaces.go -destination=../mocks/service.go -package=mocks .
package service

import (
	"context"
	"time"

	"booking-service/internal/models"

	"github.com/google/uuid"
)

type Cache[K any, V any] interface {
	Get(ctx context.Context, key K) (V, bool)
	Set(ctx context.Context, key K, value V)
	Delete(ctx context.Context, key K)
}

type SlotCahceInvalidator interface {
	InvalidateSlotsCache(ctx context.Context, roomID uuid.UUID, date time.Time)
}

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetOrCreateForDummy(ctx context.Context, user *models.User) error
}

type LaneRepository interface {
	Create(ctx context.Context, lane *models.Lane) error
	GetAll(ctx context.Context) ([]models.Lane, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Lane, error)
}

type ScheduleRepository interface {
	Create(ctx context.Context, s *models.Schedule) error
	GetByLaneID(ctx context.Context, laneID uuid.UUID) (*models.Schedule, error)
}

type SlotRepository interface {
	CreateSlots(ctx context.Context, slots []models.Slot) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Slot, error)
	GetAvailableByLaneAndDate(ctx context.Context, laneID uuid.UUID, date time.Time) ([]models.Slot, error)
	GetByLaneID(ctx context.Context, laneID uuid.UUID) ([]models.Slot, error)
}

type BookingRepository interface {
	Create(ctx context.Context, b *models.Booking) error
	Get(ctx context.Context, bookingID uuid.UUID) (*models.Booking, error)
	Cancel(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error
	GetActiveByUserID(ctx context.Context, userID uuid.UUID) ([]models.Booking, error)
	GetList(ctx context.Context, limit, page uint64) (*models.BookingList, error)
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
