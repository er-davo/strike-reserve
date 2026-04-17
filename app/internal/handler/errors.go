package handler

import (
	"booking-service/internal/api"
)

type ErrorMessage string

const (
	MsgInternalError       ErrorMessage = "Internal server error"
	MsgUnauthorized        ErrorMessage = "Authentication required"
	MsgForbiddenOnlyUsers  ErrorMessage = "Only users can create bookings"
	MsgForbiddenOnlyAdmins ErrorMessage = "Only admins can access this resource"
	MsgInvalidUserID       ErrorMessage = "Invalid user id format"

	MsgSlotNotFound      ErrorMessage = "The requested slot does not exist"
	MsgSlotAlreadyBooked ErrorMessage = "The slot is already booked by another user"

	MsgInvalidPage     ErrorMessage = "Invalid page number"
	MsgInvalidPageSize ErrorMessage = "Invalid page size"

	MsgBookingNotFound ErrorMessage = "Booking not found"
	MsgCancelForbidden ErrorMessage = "You do not have permission to cancel this booking"
	MsgAccessDenied    ErrorMessage = "Access denied"

	MsgInvalidRole    ErrorMessage = "Invalid role provided"
	MsgLoginFailed    ErrorMessage = "Failed to process login"
	MsgTokenGenFailed ErrorMessage = "Failed to generate access token"

	MsgInvalidCredentials  ErrorMessage = "Invalid email or password"
	MsgInvalidRegistration ErrorMessage = "Invalid email or password format"

	MsgInvalidDaysOfWeek ErrorMessage = "Invalid days of week"
	MsgLaneNotFound      ErrorMessage = "Lane not found"
	MsgScheduleExists    ErrorMessage = "Schedule already exists for this room"
	MsgInvalidDayOFWeek  ErrorMessage = "Invalid day of week"

	MsgInvalidDate     ErrorMessage = "Invalid date format"
	MsgInvalidLaneType ErrorMessage = "Invalid lane type"
)

func MakeError(code api.ErrorResponseErrorCode, msg ErrorMessage) struct {
	Code    api.ErrorResponseErrorCode "json:\"code\""
	Message string                     "json:\"message\""
} {
	return struct {
		Code    api.ErrorResponseErrorCode "json:\"code\""
		Message string                     "json:\"message\""
	}{
		Code:    code,
		Message: string(msg),
	}
}

func MakeInternalError() struct {
	Code    string "json:\"code\""
	Message string "json:\"message\""
} {
	return struct {
		Code    string "json:\"code\""
		Message string "json:\"message\""
	}{
		Code:    string(api.INTERNALERROR),
		Message: string(MsgInternalError),
	}
}
