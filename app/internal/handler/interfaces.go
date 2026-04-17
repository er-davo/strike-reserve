//go:generate mockgen -source=interfaces.go -destination=../mocks/handler.go -package=mocks .
package handler

import (
	"booking-service/internal/models"
	"context"
	"time"

	"github.com/google/uuid"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (*models.Claims, error)
}

type BookingService interface {
	Cancel(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) (*models.Booking, error)
	Create(ctx context.Context, userID uuid.UUID, slotID uuid.UUID) (*models.Booking, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Booking, error)
	GetList(ctx context.Context, pageSize uint64, page uint64) (*models.BookingList, error)
}

type LaneService interface {
	CreateLane(ctx context.Context, lCreate models.LaneCreate) (*models.Lane, error)
	CreateSchedule(ctx context.Context, sch *models.Schedule) error
	GetAllLanes(ctx context.Context) ([]models.Lane, error)
	GetAvailableSlots(ctx context.Context, laneID uuid.UUID, date time.Time) ([]models.Slot, error)
}

type UserService interface {
	GetOrCreateDummy(ctx context.Context, role models.UserRole) (*models.User, error)
	Login(ctx context.Context, uLogin models.UserLogin) (*models.User, error)
	Register(ctx context.Context, uRegister models.UserRegister) (*models.User, error)
}

type TokenGenerator interface {
	GenerateToken(ctx context.Context, user *models.User) (string, error)
}
