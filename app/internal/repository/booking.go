package repository

import (
	"context"
	"time"

	"booking-service/internal/models"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository struct {
	db     *pgxpool.Pool
	getter *trmpgx.CtxGetter
	psql   sq.StatementBuilderType
}

func NewBookingRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter) *BookingRepository {
	return &BookingRepository{
		db:     db,
		getter: c,
		psql:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// Create creates a new booking.
func (r *BookingRepository) Create(ctx context.Context, b *models.Booking) error {
	query, args, err := r.psql.
		Insert("bookings").
		Columns("user_id", "slot_id", "status").
		Values(b.UserID, b.SlotID, b.Status).
		Suffix("RETURNING id, created_at").
		ToSql()

	if err != nil {
		return wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	err = conn.QueryRow(ctx, query, args...).Scan(&b.ID, &b.CreatedAt)
	return wrapDBError(err)
}

// GetByID returns the booking by ID.
func (r *BookingRepository) Get(ctx context.Context, bookingID uuid.UUID) (*models.Booking, error) {
	query, args, err := r.psql.
		Select("id", "user_id", "slot_id", "status", "created_at").
		From("bookings").
		Where(sq.Eq{"id": bookingID}).
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)

	var b models.Booking
	err = conn.QueryRow(ctx, query, args...).Scan(
		&b.ID,
		&b.UserID,
		&b.SlotID,
		&b.Status,
		&b.CreatedAt,
	)
	return &b, wrapDBError(err)
}

// Cancel cancels the booking. If already cancelled, returns ErrNoRowsAffected.
func (r *BookingRepository) Cancel(ctx context.Context, bookingID, userID uuid.UUID) error {
	updateQuery, updateArgs, err := r.psql.
		Update("bookings").
		Set("status", models.StatusCancelled).
		Where(sq.Eq{"id": bookingID}).
		ToSql()
	if err != nil {
		return wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	_, err = conn.Exec(ctx, updateQuery, updateArgs...)
	return wrapDBError(err)
}

// GetActiveByUserID returns active bookings by user ID.
func (r *BookingRepository) GetActiveByUserID(ctx context.Context, userID uuid.UUID) ([]models.Booking, error) {
	query, args, err := r.psql.
		Select(
			"b.id", "b.user_id", "b.slot_id",
			"b.status", "b.created_at",
		).
		From("bookings b").
		Join("slots s ON b.slot_id = s.id").
		Where(sq.Eq{"b.user_id": userID, "b.status": models.StatusActive}).
		Where(sq.Gt{"s.start_time": time.Now().UTC()}).
		OrderBy("s.start_time ASC").
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err)
	}
	defer rows.Close()

	var result []models.Booking
	for rows.Next() {
		var b models.Booking
		err := rows.Scan(&b.ID, &b.UserID, &b.SlotID, &b.Status, &b.CreatedAt)
		if err != nil {
			return nil, wrapDBError(err)
		}
		result = append(result, b)
	}
	return result, nil
}

// GetAllPaginatedCursor returns all bookings paginated by cursor.
func (r *BookingRepository) GetList(ctx context.Context, limit, page uint64) (*models.BookingList, error) {
	conn := r.getter.DefaultTrOrDB(ctx, r.db)

	countBuilder := r.psql.Select("COUNT(*)").From("bookings")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, wrapDBError(err)
	}

	var total uint64
	err = conn.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, wrapDBError(err)
	}

	builder := r.psql.
		Select("id", "user_id", "slot_id", "status", "created_at").
		From("bookings").
		OrderBy("created_at ASC", "id ASC").
		Limit(limit).
		Offset(page * limit)

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, wrapDBError(err)
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err)
	}
	defer rows.Close()

	var result []models.Booking
	for rows.Next() {
		var b models.Booking
		err := rows.Scan(&b.ID, &b.UserID, &b.SlotID, &b.Status, &b.CreatedAt)
		if err != nil {
			return nil, wrapDBError(err)
		}
		result = append(result, b)
	}

	return &models.BookingList{
		Bookings: result,
		Total:    total,
	}, nil
}
