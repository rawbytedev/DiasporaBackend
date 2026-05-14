package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

// NewPostgresDB initializes a new PostgresDB with the given DSN (Data Source Name).
func NewPostgresDB(dsn string) (*PostgresDB, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &PostgresDB{pool: pool}, nil
}

// GetPool returns the underlying pgxpool.Pool for direct access if needed.
func (db *PostgresDB) GetPool() *pgxpool.Pool {
	return db.pool
}

func (db *PostgresDB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}

// Alive checks if the database connection is alive by pinging it.
func (db *PostgresDB) Alive() error {
	return aliveCheck(db.pool)
}

// Close closes the database connection pool.
func aliveCheck(pool *pgxpool.Pool) error {
	return pool.Ping(context.Background())
}

// Close closes the database connection pool.
func (db *PostgresDB) Close() {
	db.pool.Close()
}
