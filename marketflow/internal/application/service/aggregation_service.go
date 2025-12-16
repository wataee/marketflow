package service

import (
	"context"
	"errors"
	"log/slog"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"os"
	"strings"
	"sync"
	"time"
)

type AggregationService struct {
	cache        port.CachePort
	storage      port.StoragePort
	logger       *slog.Logger
	ticker       *time.Ticker
	done         chan struct{}
	tradingPairs []string
	exchanges    []string
	retention    time.Duration
	mu           sync.RWMutex
}

func NewAggregationService(cache port.CachePort, storage port.StoragePort, logger *slog.Logger) *AggregationService {
	s := &AggregationService{
		cache:   cache,
		storage: storage,
		logger:  logger,
		done:    make(chan struct{}),
	}

	if v := os.Getenv("TRADING_PAIRS"); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				s.tradingPairs = append(s.tradingPairs, p)
			}
		}
	}

	return s
}

func (s *AggregationService) SetTradingPairs(pairs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tradingPairs = append([]string{}, pairs...)
	s.logger.Info("trading pairs set", "count", len(pairs), "pairs", pairs)
}

func (s *AggregationService) SetExchanges(exchanges []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exchanges = append([]string{}, exchanges...)
	s.logger.Info("exchanges set for aggregation", "count", len(exchanges), "exchanges", exchanges)
}

func (s *AggregationService) Start(ctx context.Context, interval time.Duration) {
	s.mu.Lock()
	if interval <= 0 {
		interval = time.Minute
	}
	s.retention = interval
	s.ticker = time.NewTicker(interval)
	s.mu.Unlock()

	s.logger.Info("aggregation service starting", "interval", interval)
	go s.aggregateLoop(ctx)
}

func (s *AggregationService) Stop() {
	s.mu.Lock()
	if s.ticker != nil {
		s.ticker.Stop()
		s.logger.Info("aggregation ticker stopped")
	}
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	s.mu.Unlock()
	s.logger.Info("aggregation service stopped")
}

func (s *AggregationService) aggregateLoop(ctx context.Context) {
	s.logger.Info("aggregation loop started")
	for {
		select {
		case <-s.ticker.C:
			s.logger.Info("starting aggregation cycle")
			start := time.Now()
			if err := s.aggregateAndStore(ctx); err != nil {
				s.logger.Error("aggregation failed", "error", err, "duration", time.Since(start))
			} else {
				s.logger.Info("aggregation cycle completed", "duration", time.Since(start))
			}
		case <-s.done:
			s.logger.Info("aggregation loop stopping")
			return
		case <-ctx.Done():
			s.logger.Info("aggregation loop cancelled")
			return
		}
	}
}

func (s *AggregationService) aggregateAndStore(ctx context.Context) error {
	s.mu.RLock()
	pairs := append([]string{}, s.tradingPairs...)
	exchanges := append([]string{}, s.exchanges...)
	retention := s.retention
	s.mu.RUnlock()

	if len(pairs) == 0 {
		s.logger.Warn("no trading pairs configured for aggregation")
		return errors.New("no trading pairs configured for aggregation")
	}

	s.logger.Info("aggregating data", "pairs", len(pairs), "exchanges", len(exchanges))

	var batch []model.AggregatedPrice

	// Если exchanges заданы, агрегируем по каждому exchange отдельно
	if len(exchanges) > 0 {
		for _, exchange := range exchanges {
			for _, pair := range pairs {
				prices, err := s.cache.GetPricesInWindow(ctx, pair, exchange)
				if err != nil {
					s.logger.Error("failed to get prices from cache",
						"pair", pair,
						"exchange", exchange,
						"error", err)
					continue
				}

				if len(prices) == 0 {
					s.logger.Debug("no prices in window",
						"pair", pair,
						"exchange", exchange)
					continue
				}

				avg, mn, mx := computeStats(prices)

				ap := model.AggregatedPrice{
					PairName:     pair,
					Exchange:     exchange,
					Timestamp:    time.Now().UTC(),
					AveragePrice: avg,
					MinPrice:     mn,
					MaxPrice:     mx,
				}
				batch = append(batch, ap)

				s.logger.Debug("aggregated prices for pair",
					"pair", pair,
					"exchange", exchange,
					"count", len(prices),
					"avg", avg,
					"min", mn,
					"max", mx)
			}
		}
	} else {
		// Если exchanges не заданы, агрегируем по всем
		for _, pair := range pairs {
			prices, err := s.cache.GetPricesInWindow(ctx, pair, "")
			if err != nil {
				s.logger.Error("failed to get prices from cache",
					"pair", pair,
					"error", err)
				continue
			}

			if len(prices) == 0 {
				s.logger.Debug("no prices in window", "pair", pair)
				continue
			}

			// Группируем по exchange
			pricesByExchange := make(map[string][]model.PriceUpdate)
			for _, p := range prices {
				pricesByExchange[p.Exchange] = append(pricesByExchange[p.Exchange], p)
			}

			for exchange, exPrices := range pricesByExchange {
				avg, mn, mx := computeStats(exPrices)

				ap := model.AggregatedPrice{
					PairName:     pair,
					Exchange:     exchange,
					Timestamp:    time.Now().UTC(),
					AveragePrice: avg,
					MinPrice:     mn,
					MaxPrice:     mx,
				}
				batch = append(batch, ap)

				s.logger.Debug("aggregated prices for pair",
					"pair", pair,
					"exchange", exchange,
					"count", len(exPrices),
					"avg", avg,
					"min", mn,
					"max", mx)
			}
		}
	}

	if len(batch) == 0 {
		s.logger.Info("aggregation: nothing to store")
		if err := s.cache.DeleteOldPrices(ctx, time.Now().Add(-retention)); err != nil {
			s.logger.Error("failed to delete old prices from cache", "error", err)
		} else {
			s.logger.Info("old prices deleted from cache")
		}
		return nil
	}

	s.logger.Info("saving aggregated batch", "count", len(batch))
	if err := s.storage.SaveAggregatedPrices(ctx, batch); err != nil {
		s.logger.Error("failed to save aggregated prices", "error", err, "batch_size", len(batch))
		return err
	}

	s.logger.Info("aggregated batch saved successfully", "count", len(batch))

	if err := s.cache.DeleteOldPrices(ctx, time.Now().Add(-retention)); err != nil {
		s.logger.Error("failed to delete old prices from cache", "error", err)
	} else {
		s.logger.Debug("old prices deleted from cache")
	}

	return nil
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