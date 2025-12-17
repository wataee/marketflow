package port

import (
	"context"
	"time"

	"marketflow/internal/domain/model"
)

type StoragePort interface {
	SaveAggregatedPrices(ctx context.Context, prices []model.AggregatedPrice) error
	GetHighestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error)
	GetLowestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error)
	GetAveragePrice(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error)
	Ping(ctx context.Context) error
	Close() error
}
