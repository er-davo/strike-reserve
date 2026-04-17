package database

import (
	"context"

	"booking-service/internal/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Connect establishes a connection pool to the PostgreSQL database and verifies it with a ping.
func Connect(ctx context.Context, db string) (*pgxpool.Pool, error) {
	conn, err := pgxpool.New(ctx, db)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	return conn, nil
}

// ConnectWithConfig establishes a connection pool with config to the PostgreSQL database and verifies it with a ping.
func ConnectWithConfig(ctx context.Context, db config.Database) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(db.Dsn)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = db.Conn.Max
	cfg.MinConns = db.Conn.Min

	cfg.MaxConnLifetime = db.Conn.MaxLifeTime
	cfg.MaxConnIdleTime = db.Conn.MaxIdleTime

	cfg.ConnConfig.ConnectTimeout = db.Conn.ConnectTimeout

	conn, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	return conn, nil
}

// Migrate runs database migrations from the given directory.
// Ignores ErrNoChange if there are no new migrations to apply.
func Migrate(migrationDir, db string) error {
	m, err := migrate.New("file://"+migrationDir, db)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
