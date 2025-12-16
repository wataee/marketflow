package usecase

import (
	"context"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"time"
)

type PriceUseCase struct {
	storage port.StoragePort
	cache   port.CachePort
}

func NewPriceUseCase(storage port.StoragePort, cache port.CachePort) *PriceUseCase {
	return &PriceUseCase{
		storage: storage,
		cache:   cache,
	}
}

// GetLatestPrice сначала пытается из Redis, потом из Postgres
func (uc *PriceUseCase) GetLatestPrice(ctx context.Context, symbol, exchange string) (*model.LatestPrice, error) {
	price, err := uc.cache.GetLatestPrice(ctx, symbol, exchange)
	if err == nil && price != nil {
		return price, nil
	}
	// fallback на Postgres, берём за последний 1 мин
	p, err := uc.storage.GetAveragePrice(ctx, symbol, exchange, time.Minute)
	if err != nil {
		return nil, err
	}
	if p == 0 {
		return nil, nil
	}
	return &model.LatestPrice{
		Symbol:   symbol,
		Exchange: exchange,
		Price:    p,
	}, nil
}

func (uc *PriceUseCase) GetHighestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	return uc.storage.GetHighestPrice(ctx, symbol, exchange, period)
}

func (uc *PriceUseCase) GetLowestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	return uc.storage.GetLowestPrice(ctx, symbol, exchange, period)
}

func (uc *PriceUseCase) GetAveragePrice(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error) {
	return uc.storage.GetAveragePrice(ctx, symbol, exchange, period)
}
