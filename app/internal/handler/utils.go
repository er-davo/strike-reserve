package handler

import (
	"context"

	"booking-service/internal/api"
	"booking-service/internal/models"

	"github.com/oapi-codegen/runtime/types"
)

func getUser(ctx context.Context) (*models.Claims, bool) {
	user, ok := ctx.Value(UserKey).(*models.Claims)
	return user, ok
}

func toApiBooking(booking models.Booking) *api.Booking {
	return &api.Booking{
		Id:     booking.ID,
		UserId: booking.UserID,
		SlotId: booking.SlotID,
		Status: api.BookingStatus(booking.Status),
	}
}

func toApiRoom(lane models.Lane) *api.Lane {
	return &api.Lane{
		Id:          lane.ID,
		Name:        lane.Name,
		Type:        ptr(api.LaneType(lane.Type)),
		IsActive:    lane.IsActive,
		Description: lane.Description,
		CreatedAt:   &lane.CreatedAt,
	}
}

func toApiSlot(slot models.Slot) *api.Slot {
	return &api.Slot{
		Id:     slot.ID,
		LaneId: slot.LaneID,
		Start:  slot.StartTime,
		End:    slot.EndTime,
	}
}

func toApiUser(user models.User) *api.User {
	return &api.User{
		Id:        user.ID,
		Email:     types.Email(user.Email),
		Role:      api.UserRole(user.Role),
		CreatedAt: &user.CreatedAt,
	}
}

func toApiSchedule(schedule models.Schedule) *api.Schedule {
	return &api.Schedule{
		Id:         &schedule.ID,
		DaysOfWeek: schedule.DaysOfWeek,
		LaneId:     schedule.LaneID,
		StartTime:  schedule.StartTime.HMTime(),
		EndTime:    schedule.EndTime.HMTime(),
	}
}

func ptr[T any](v T) *T {
	return &v
}
