package port

import (
	"context"
	"marketflow/internal/domain/model"
	"time"
)

// StoragePort определяет интерфейс для работы с хранилищем
type StoragePort interface {
	SaveAggregatedPrices(ctx context.Context, prices []model.AggregatedPrice) error
	GetHighestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error)
	GetLowestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error)
	GetAveragePrice(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error)
	Close() error
}
