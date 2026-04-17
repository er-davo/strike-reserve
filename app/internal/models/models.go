package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims defines jwt claims
type Claims struct {
	UserID string   `json:"user_id"`
	Role   UserRole `json:"role"`
	jwt.RegisteredClaims
}

// UserRole defines the access level of a user
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

func (r UserRole) Valid() bool {
	switch r {
	case RoleAdmin:
		return true
	case RoleUser:
		return true
	default:
		return false
	}
}

// BookingStatus defines the current state of a reservation
type BookingStatus string

const (
	StatusActive    BookingStatus = "active"
	StatusCancelled BookingStatus = "cancelled"
)

type LaneType string

const (
	LaneTypeStandard = "standard"
	LaneTypeVip      = "vip"
	LaneTypePro      = "pro"
	LaneTypeKids     = "kids"
)

func (t LaneType) Valid() bool {
	switch t {
	case LaneTypeStandard:
		return true
	case LaneTypeVip:
		return true
	case LaneTypePro:
		return true
	case LaneTypeKids:
		return true
	default:
		return false
	}
}

func (s BookingStatus) Valid() bool {
	switch s {
	case StatusActive:
		return true
	case StatusCancelled:
		return true
	default:
		return false
	}
}

// User represents a system participant
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash *string   `json:"-"`
	Role         UserRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserRegister defines the input for creating a User
type UserRegister struct {
	Email    string   `json:"email" validate:"required,email"`
	Password string   `json:"password" validate:"required,min=8"`
	Role     UserRole `json:"role"`
}

// UserLogin defines the input for logging in a User
type UserLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// Lane represents a physical bowling lane
type Lane struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        LaneType  `json:"type"`
	Description *string   `json:"description,omitempty"`
	IsActive    *bool     `json:"is_active,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type LaneCreate struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Type        LaneType `json:"type"`
}

// Schedule defines the availability rules for a Room
type Schedule struct {
	ID         uuid.UUID `json:"id"`
	LaneID     uuid.UUID `json:"room_id" validate:"required"`
	DaysOfWeek []int     `json:"day_of_week" validate:"required,min=1,max=7,unique,dive,min=1,max=7"` // 1 (Mon) to 7 (Sun)
	StartTime  HMSTime   `json:"start_time" validate:"required"`                                      // Format: "HH:MM:SS"
	EndTime    HMSTime   `json:"end_time" validate:"required"`                                        // Format: "HH:MM:SS"
	CreatedAt  time.Time `json:"created_at"`
}

// Slot represents a generated 30-minute time window
type Slot struct {
	ID        uuid.UUID `json:"id"`
	LaneID    uuid.UUID `json:"room_id"`
	StartTime time.Time `json:"start_at"`
	EndTime   time.Time `json:"end_at"`
}

// Booking represents a relationship between a User and a Slot
type Booking struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"user_id"`
	SlotID    uuid.UUID     `json:"slot_id"`
	Status    BookingStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
}

type BookingList struct {
	Bookings []Booking `json:"bookings"`
	Total    uint64    `json:"total"`
}
