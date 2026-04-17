package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Exported errors for Service layer
var (
	ErrNotFound       = errors.New("resource not found")
	ErrConflict       = errors.New("resource already exists or conflicts")
	ErrForeignKey     = errors.New("referenced resource does not exist")
	ErrValidation     = errors.New("database validation failed")
	ErrInternal       = errors.New("internal database error")
	ErrNoRowsAffected = errors.New("no rows affected")
	ErrForbidden      = errors.New("this action is forbidden")
)

// Postgres error codes constants
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
	pgNotNullViolation    = "23502"
	pgExclusionViolation  = "23P01"
)

func wrapDBError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation, pgExclusionViolation:
			return fmt.Errorf("%w: %s", ErrConflict, pgErr.Detail)
		case pgForeignKeyViolation:
			return fmt.Errorf("%w: %s", ErrForeignKey, pgErr.Detail)
		case pgCheckViolation, pgNotNullViolation:
			return fmt.Errorf("%w: %s", ErrValidation, pgErr.Message)
		default:
			// Unhandled postgres error
			return fmt.Errorf("%w (pg code %s): %v", ErrInternal, pgErr.Code, pgErr.Message)
		}
	}

	// Fallback for other errors (connection, etc.)
	return fmt.Errorf("%w: %v", ErrInternal, err)
}
