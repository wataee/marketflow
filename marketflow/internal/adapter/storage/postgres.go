package storage

import (
	"context"
	"database/sql"
	"fmt"
	"marketflow/internal/domain/model"
	"time"

	_ "github.com/lib/pq"
)

type PostgresAdapter struct {
	db *sql.DB
}

func NewPostgresAdapter(connStr string) (*PostgresAdapter, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresAdapter{db: db}, nil
}

func (a *PostgresAdapter) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS aggregated_prices (
		id SERIAL PRIMARY KEY,
		pair_name VARCHAR(20) NOT NULL,
		exchange VARCHAR(50) NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		average_price DOUBLE PRECISION NOT NULL,
		min_price DOUBLE PRECISION NOT NULL,
		max_price DOUBLE PRECISION NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_pair_exchange_timestamp ON aggregated_prices(pair_name, exchange, timestamp);
	`
	_, err := a.db.ExecContext(ctx, query)
	return err
}

func (a *PostgresAdapter) SaveAggregatedPrices(ctx context.Context, prices []model.AggregatedPrice) error {
	// TODO: Implement batch insert
	return nil
}

func (a *PostgresAdapter) GetHighestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	// TODO: Implement
	return nil, nil
}

func (a *PostgresAdapter) GetLowestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	// TODO: Implement
	return nil, nil
}

func (a *PostgresAdapter) GetAveragePrice(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error) {
	// TODO: Implement
	return 0, nil
}

func (a *PostgresAdapter) Close() error {
	return a.db.Close()
}
