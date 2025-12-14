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

func (uc *PriceUseCase) GetLatestPrice(ctx context.Context, symbol, exchange string) (*model.LatestPrice, error) {
	// Сначала проверяем кеш
	price, err := uc.cache.GetLatestPrice(ctx, symbol, exchange)
	if err == nil && price != nil {
		return price, nil
	}

	// Если в кеше нет, пытаемся получить из БД
	// TODO: implement fallback to storage
	return nil, err
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
