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

type LaneRepository struct {
	db     *pgxpool.Pool
	getter *trmpgx.CtxGetter
	psql   sq.StatementBuilderType
}

func NewLaneRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter) *LaneRepository {
	return &LaneRepository{
		db:     db,
		getter: c,
		psql:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// Create inserts a new lane into the database
func (r *LaneRepository) Create(ctx context.Context, lane *models.Lane) error {
	if lane.CreatedAt.IsZero() {
		lane.CreatedAt = time.Now().UTC()
	}

	query, args, err := r.psql.
		Insert("lanes").
		Columns(
			"name", "description", "type",
			"is_active", "created_at",
		).
		Values(lane.Name, lane.Description, lane.Type, lane.IsActive, lane.CreatedAt).
		Suffix("RETURNING id").
		ToSql()

	if err != nil {
		return wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	err = conn.QueryRow(ctx, query, args...).Scan(&lane.ID)
	return wrapDBError(err)
}

// GetAll returns all lanes in the system
func (r *LaneRepository) GetAll(ctx context.Context) ([]models.Lane, error) {
	query, args, err := r.psql.
		Select(
			"id", "name", "description",
			"type", "is_active", "created_at",
		).
		From("lanes").
		OrderBy("name ASC").
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

	lanes := make([]models.Lane, 0, 50)

	for rows.Next() {
		var ln models.Lane
		err := rows.Scan(
			&ln.ID,
			&ln.Name,
			&ln.Description,
			&ln.Type,
			&ln.IsActive,
			&ln.CreatedAt,
		)
		if err != nil {
			return nil, wrapDBError(err)
		}
		lanes = append(lanes, ln)
	}

	if err = rows.Err(); err != nil {
		return nil, wrapDBError(err)
	}

	return lanes, nil
}

// Get finds a specific room by its UUID
func (r *LaneRepository) Get(ctx context.Context, id uuid.UUID) (*models.Lane, error) {
	query, args, err := r.psql.
		Select(
			"id", "name", "description", "type",
			"is_active", "created_at",
		).
		From("lanes").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	rm := &models.Lane{}
	err = conn.QueryRow(ctx, query, args...).Scan(
		&rm.ID,
		&rm.Name,
		&rm.Description,
		&rm.Type,
		&rm.IsActive,
		&rm.CreatedAt,
	)
	if err != nil {
		return nil, wrapDBError(err)
	}

	return rm, nil
}
