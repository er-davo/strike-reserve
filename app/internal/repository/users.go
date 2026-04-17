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

type UserRepository struct {
	db     *pgxpool.Pool
	getter *trmpgx.CtxGetter
	psql   sq.StatementBuilderType
}

func NewUserRepository(db *pgxpool.Pool, c *trmpgx.CtxGetter) *UserRepository {
	return &UserRepository{
		db:     db,
		getter: c,
		psql:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// GetOrCreateForDummy handles the fixed UUID login for admin/user roles
func (r *UserRepository) GetOrCreateForDummy(ctx context.Context, user *models.User) error {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}

	query, args, err := r.psql.
		Insert("users").
		Columns("id", "email", "role", "created_at").
		Values(user.ID, user.Email, string(user.Role), user.CreatedAt).
		Suffix("ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role RETURNING created_at").
		ToSql()

	if err != nil {
		return wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	err = conn.QueryRow(ctx, query, args...).Scan(&user.CreatedAt)
	if err != nil {
		return wrapDBError(err)
	}

	return nil
}

// Create inserts a new user with a password hash (for standard registration)
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}

	query, args, err := r.psql.
		Insert("users").
		Columns("id", "email", "password_hash", "role", "created_at").
		Values(user.ID, user.Email, user.PasswordHash, string(user.Role), user.CreatedAt).
		ToSql()

	if err != nil {
		return wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	_, err = conn.Exec(ctx, query, args...)
	return wrapDBError(err)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query, args, err := r.psql.
		Select("id", "email", "password_hash", "role", "created_at").
		From("users").
		Where(sq.Eq{"email": email}).
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	user := &models.User{}

	err = conn.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, wrapDBError(err)
	}

	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query, args, err := r.psql.
		Select("id", "email", "role", "created_at").
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, wrapDBError(err)
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.db)
	user := &models.User{}
	err = conn.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, wrapDBError(err)
	}

	return user, nil
}
