package repository

import (
	"context"
	"time"

	"booking-service/internal/models"

	sq "github.com/Masterminds/squirrel"
	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SlotRepository struct {
	db     *pgxpool.Pool
	getter *trmpgx.CtxGetter
	psql   sq.StatementBuilderType
}

func NewSlotRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter) *SlotRepository {
	return &SlotRepository{
		db:     db,
		getter: c,
		psql:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// CreateSlots inserts multiple slots efficiently using pgx.Batch
func (r *SlotRepository) CreateSlots(ctx context.Context, slots []models.Slot) error {
	if len(slots) == 0 {
		return nil
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	batch := &pgx.Batch{}

	for _, s := range slots {
		query, args, err := r.psql.
			Insert("slots").
			Columns("id", "lane_id", "start_time", "end_time").
			Values(s.ID, s.LaneID, s.StartTime, s.EndTime).
			ToSql()
		if err != nil {
			return wrapDBError(err)
		}
		batch.Queue(query, args...)
	}

	br := conn.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(slots); i++ {
		_, err := br.Exec()
		if err != nil {
			return wrapDBError(err)
		}
	}

	return nil
}

// GetByID returns a slot by its ID
func (r *SlotRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Slot, error) {
	query, args, err := r.psql.
		Select("id", "lane_id", "start_time", "end_time").
		From("slots").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	var s models.Slot
	err = r.getter.DefaultTrOrDB(ctx, r.db).
		QueryRow(ctx, query, args...).
		Scan(&s.ID, &s.LaneID, &s.StartTime, &s.EndTime)
	if err != nil {
		return nil, wrapDBError(err)
	}

	return &s, nil
}

// GetByLaneID returns all slots for a given room ID
func (r *SlotRepository) GetByLaneID(ctx context.Context, laneID uuid.UUID) ([]models.Slot, error) {
	query, args, err := r.psql.
		Select("id", "lane_id", "start_time", "end_time").
		From("slots").
		Where(sq.Eq{"lane_id": laneID}).
		OrderBy("start_time ASC").
		ToSql()

	if err != nil {
		return nil, err
	}

	rows, err := r.getter.DefaultTrOrDB(ctx, r.db).Query(ctx, query, args...)
	if err != nil {
		return nil, wrapDBError(err)
	}
	defer rows.Close()

	var slots []models.Slot
	for rows.Next() {
		var s models.Slot
		if err := rows.Scan(&s.ID, &s.LaneID, &s.StartTime, &s.EndTime); err != nil {
			return nil, wrapDBError(err)
		}
		slots = append(slots, s)
	}

	return slots, nil
}

// GetAvailableByLaneAndDate returns slots that don't have an active booking
func (r *SlotRepository) GetAvailableByLaneAndDate(ctx context.Context, laneID uuid.UUID, date time.Time) ([]models.Slot, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	query, args, err := r.psql.
		Select("s.id", "s.lane_id", "s.start_time", "s.end_time").
		From("slots s").
		LeftJoin("bookings b ON s.id = b.slot_id AND b.status = 'active'").
		Where(sq.Eq{"s.lane_id": laneID}).
		Where(sq.GtOrEq{"s.start_time": startOfDay}).
		Where(sq.Lt{"s.start_time": endOfDay}).
		Where(sq.Expr("b.id IS NULL")).
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

	var slots []models.Slot
	for rows.Next() {
		var s models.Slot
		err := rows.Scan(&s.ID, &s.LaneID, &s.StartTime, &s.EndTime)
		if err != nil {
			return nil, wrapDBError(err)
		}
		slots = append(slots, s)
	}

	if err = rows.Err(); err != nil {
		return nil, wrapDBError(err)
	}

	return slots, nil
}
