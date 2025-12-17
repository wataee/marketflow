package port

import (
	"context"
	"time"

	"marketflow/internal/domain/model"
)

type CachePort interface {
	SetLatestPrice(ctx context.Context, price model.PriceUpdate) error
	GetLatestPrice(ctx context.Context, symbol, exchange string) (*model.LatestPrice, error)
	GetLatestPriceAny(ctx context.Context, symbol string) (*model.LatestPrice, error)
	AddPriceToWindow(ctx context.Context, price model.PriceUpdate) error
	GetPricesInWindow(ctx context.Context, symbol, exchange string) ([]model.PriceUpdate, error)
	DeleteOldPrices(ctx context.Context, before time.Time) error
	Ping(ctx context.Context) error
	Close() error
}
