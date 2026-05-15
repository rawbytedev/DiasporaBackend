package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns = 5
	defaultMinConns = 1
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

// NewPostgresDB initializes a new PostgresDB with the given DSN (Data Source Name).
func NewPostgresDB(dsn string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = defaultMaxConns
	config.MinConns = defaultMinConns
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &PostgresDB{pool: pool}, nil
}

// NewPostgresDBWithConfig initializes PostgresDB with custom pool config
func NewPostgresDBWithConfig(dsn string, maxConns int32, minConns int32) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if maxConns < 1 {
		maxConns = defaultMaxConns
	}
	if minConns < 0 {
		minConns = 0
	}
	if minConns > maxConns {
		minConns = 0
	}
	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
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
	if db.pool != nil {
		db.pool.Close()
	}
}
