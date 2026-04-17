package service

import "errors"

var (
	ErrNotFound               = errors.New("not found")
	ErrSlotNotExisting        = errors.New("slot not existing")
	ErrSlotIsUsed             = errors.New("slot is used")
	ErrForbidden              = errors.New("forbidden")
	ErrTokenIsInvalid         = errors.New("token invalid")
	ErrAlreadyExists          = errors.New("already exists")
	ErrInvalidEmailOrPassword = errors.New("invalid email or password")
	ErrLaneNotFound           = errors.New("room not found")
)
