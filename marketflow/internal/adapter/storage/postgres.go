package storage

import (
	"context"
	"database/sql"
	"fmt"
	"marketflow/internal/domain/model"
	"strings"
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

// Ping реализует порт Ping
func (a *PostgresAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

func (a *PostgresAdapter) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS aggregated_prices (
		id SERIAL PRIMARY KEY,
		pair_name VARCHAR(50) NOT NULL,
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
	if len(prices) == 0 {
		return nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Собираем placeholders и args
	valueStrings := make([]string, 0, len(prices))
	valueArgs := make([]interface{}, 0, len(prices)*5)
	for i, p := range prices {
		// 5 полей на запись: pair_name, exchange, timestamp, average_price, min_price, max_price
		n := i*6 + 1
		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", n, n+1, n+2, n+3, n+4, n+5))
		valueArgs = append(valueArgs, p.PairName, p.Exchange, p.Timestamp, p.AveragePrice, p.MinPrice, p.MaxPrice)
	}

	stmt := fmt.Sprintf("INSERT INTO aggregated_prices (pair_name, exchange, timestamp, average_price, min_price, max_price) VALUES %s", strings.Join(valueStrings, ","))
	if _, err := tx.ExecContext(ctx, stmt, valueArgs...); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (a *PostgresAdapter) GetHighestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	since := time.Now().Add(-period)
	var row *sql.Row
	if exchange == "" {
		query := `SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM aggregated_prices
			WHERE pair_name = $1 AND timestamp >= $2
			ORDER BY average_price DESC LIMIT 1`
		row = a.db.QueryRowContext(ctx, query, symbol, since)
	} else {
		query := `SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM aggregated_prices
			WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3
			ORDER BY average_price DESC LIMIT 1`
		row = a.db.QueryRowContext(ctx, query, symbol, exchange, since)
	}

	var ap model.AggregatedPrice
	if err := row.Scan(&ap.PairName, &ap.Exchange, &ap.Timestamp, &ap.AveragePrice, &ap.MinPrice, &ap.MaxPrice); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ap, nil
}

func (a *PostgresAdapter) GetLowestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	since := time.Now().Add(-period)
	var row *sql.Row
	if exchange == "" {
		query := `SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM aggregated_prices
			WHERE pair_name = $1 AND timestamp >= $2
			ORDER BY average_price ASC LIMIT 1`
		row = a.db.QueryRowContext(ctx, query, symbol, since)
	} else {
		query := `SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM aggregated_prices
			WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3
			ORDER BY average_price ASC LIMIT 1`
		row = a.db.QueryRowContext(ctx, query, symbol, exchange, since)
	}

	var ap model.AggregatedPrice
	if err := row.Scan(&ap.PairName, &ap.Exchange, &ap.Timestamp, &ap.AveragePrice, &ap.MinPrice, &ap.MaxPrice); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ap, nil
}

func (a *PostgresAdapter) GetAveragePrice(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error) {
	since := time.Now().Add(-period)
	var query string
	var row *sql.Row
	if exchange == "" {
		query = `SELECT AVG(average_price) FROM aggregated_prices WHERE pair_name = $1 AND timestamp >= $2`
		row = a.db.QueryRowContext(ctx, query, symbol, since)
	} else {
		query = `SELECT AVG(average_price) FROM aggregated_prices WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3`
		row = a.db.QueryRowContext(ctx, query, symbol, exchange, since)
	}

	var avg sql.NullFloat64
	if exchange == "" {
		if err := row.Scan(&avg); err != nil {
			return 0, err
		}
	} else {
		if err := row.Scan(&avg); err != nil {
			return 0, err
		}
	}

	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

func (a *PostgresAdapter) Close() error {
	return a.db.Close()
}
