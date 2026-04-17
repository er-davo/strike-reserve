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

type ScheduleRepository struct {
	db     *pgxpool.Pool
	getter *trmpgx.CtxGetter
	psql   sq.StatementBuilderType
}

func NewScheduleRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter) *ScheduleRepository {
	return &ScheduleRepository{
		db:     db,
		getter: c,
		psql:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// Create inserts a new schedule rule for a room
func (r *ScheduleRepository) Create(ctx context.Context, s *models.Schedule) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}

	query, args, err := r.psql.
		Insert("schedules").
		Columns("lane_id", "days_of_week", "start_time", "end_time", "created_at").
		Values(s.LaneID, s.DaysOfWeek, s.StartTime, s.EndTime, s.CreatedAt).
		Suffix("RETURNING id").
		ToSql()

	if err != nil {
		return wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	err = conn.QueryRow(ctx, query, args...).Scan(&s.ID)
	return wrapDBError(err)
}

// GetByRoomID returns schedule rules for a specific room
func (r *ScheduleRepository) GetByLaneID(ctx context.Context, laneID uuid.UUID) (*models.Schedule, error) {
	query, args, err := r.psql.
		Select("id", "lane_id", "days_of_week", "start_time", "end_time", "created_at").
		From("schedules").
		Where(sq.Eq{"lane_id": laneID}).
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	var schedule models.Schedule

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	err = conn.QueryRow(ctx, query, args...).Scan(
		&schedule.ID,
		&schedule.LaneID,
		&schedule.DaysOfWeek,
		&schedule.StartTime,
		&schedule.EndTime,
		&schedule.CreatedAt,
	)
	if err != nil {
		return nil, wrapDBError(err)
	}

	return &schedule, nil
}
