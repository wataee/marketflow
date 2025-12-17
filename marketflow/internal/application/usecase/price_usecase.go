package usecase

import (
	"context"
	"time"

	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
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
	if exchange != "" {
		price, err := uc.cache.GetLatestPrice(ctx, symbol, exchange)
		if err == nil && price != nil {
			return price, nil
		}
	} else {
		price, err := uc.cache.GetLatestPriceAny(ctx, symbol)
		if err == nil && price != nil {
			return price, nil
		}
	}

	avgPrice, err := uc.storage.GetAveragePrice(ctx, symbol, exchange, time.Minute)
	if err != nil {
		return nil, err
	}
	if avgPrice == 0 {
		return nil, nil
	}

	return &model.LatestPrice{
		Symbol:    symbol,
		Exchange:  exchange,
		Price:     avgPrice,
		Timestamp: time.Now(),
	}, nil
}

func (uc *PriceUseCase) GetHighestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	price, err := uc.storage.GetHighestPrice(ctx, symbol, exchange, period)
	if err == nil && price != nil {
		return price, nil
	}

	return uc.computeHighestFromCache(ctx, symbol, exchange, period)
}

func (uc *PriceUseCase) GetLowestPrice(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	price, err := uc.storage.GetLowestPrice(ctx, symbol, exchange, period)
	if err == nil && price != nil {
		return price, nil
	}

	return uc.computeLowestFromCache(ctx, symbol, exchange, period)
}

func (uc *PriceUseCase) GetAveragePrice(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error) {
	avgPrice, err := uc.storage.GetAveragePrice(ctx, symbol, exchange, period)
	if err == nil && avgPrice > 0 {
		return avgPrice, nil
	}

	return uc.computeAverageFromCache(ctx, symbol, exchange, period)
}

func (uc *PriceUseCase) computeHighestFromCache(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	prices, err := uc.getPricesFromCache(ctx, symbol, exchange, period)
	if err != nil || len(prices) == 0 {
		return nil, nil
	}

	maxPrice := prices[0].Price
	var maxPriceUpdate model.PriceUpdate
	for _, p := range prices {
		if p.Price > maxPrice {
			maxPrice = p.Price
			maxPriceUpdate = p
		}
	}

	avg, min, max := computeStats(prices)

	return &model.AggregatedPrice{
		PairName:     symbol,
		Exchange:     maxPriceUpdate.Exchange,
		Timestamp:    maxPriceUpdate.Timestamp,
		AveragePrice: avg,
		MinPrice:     min,
		MaxPrice:     max,
	}, nil
}

func (uc *PriceUseCase) computeLowestFromCache(ctx context.Context, symbol, exchange string, period time.Duration) (*model.AggregatedPrice, error) {
	prices, err := uc.getPricesFromCache(ctx, symbol, exchange, period)
	if err != nil || len(prices) == 0 {
		return nil, nil
	}

	minPrice := prices[0].Price
	var minPriceUpdate model.PriceUpdate
	for _, p := range prices {
		if p.Price < minPrice {
			minPrice = p.Price
			minPriceUpdate = p
		}
	}

	avg, min, max := computeStats(prices)

	return &model.AggregatedPrice{
		PairName:     symbol,
		Exchange:     minPriceUpdate.Exchange,
		Timestamp:    minPriceUpdate.Timestamp,
		AveragePrice: avg,
		MinPrice:     min,
		MaxPrice:     max,
	}, nil
}

func (uc *PriceUseCase) computeAverageFromCache(ctx context.Context, symbol, exchange string, period time.Duration) (float64, error) {
	prices, err := uc.getPricesFromCache(ctx, symbol, exchange, period)
	if err != nil || len(prices) == 0 {
		return 0, nil
	}

	avg, _, _ := computeStats(prices)
	return avg, nil
}

func (uc *PriceUseCase) getPricesFromCache(ctx context.Context, symbol, exchange string, period time.Duration) ([]model.PriceUpdate, error) {
	allPrices, err := uc.cache.GetPricesInWindow(ctx, symbol, exchange)
	if err != nil {
		return nil, err
	}

	if period >= time.Minute {
		return allPrices, nil
	}

	cutoff := time.Now().Add(-period)
	var filtered []model.PriceUpdate
	for _, p := range allPrices {
		if p.Timestamp.After(cutoff) {
			filtered = append(filtered, p)
		}
	}

	return filtered, nil
}

func computeStats(prices []model.PriceUpdate) (avg, min, max float64) {
	if len(prices) == 0 {
		return 0, 0, 0
	}
	min = prices[0].Price
	max = prices[0].Price
	var sum float64
	for _, p := range prices {
		if p.Price < min {
			min = p.Price
		}
		if p.Price > max {
			max = p.Price
		}
		sum += p.Price
	}
	avg = sum / float64(len(prices))
	return avg, min, max
}
