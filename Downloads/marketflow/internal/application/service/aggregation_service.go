package service

import (
	"context"
	"log/slog"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"time"
)

type AggregationService struct {
	cache   port.CachePort
	storage port.StoragePort
	logger  *slog.Logger
	ticker  *time.Ticker
	done    chan struct{}
}

func NewAggregationService(cache port.CachePort, storage port.StoragePort, logger *slog.Logger) *AggregationService {
	return &AggregationService{
		cache:   cache,
		storage: storage,
		logger:  logger,
		done:    make(chan struct{}),
	}
}

func (s *AggregationService) Start(ctx context.Context, interval time.Duration) {
	s.ticker = time.NewTicker(interval)
	go s.aggregateLoop(ctx)
}

func (s *AggregationService) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.done)
}

func (s *AggregationService) aggregateLoop(ctx context.Context) {
	for {
		select {
		case <-s.ticker.C:
			if err := s.aggregateAndStore(ctx); err != nil {
				s.logger.Error("aggregation failed", "error", err)
			}
		case <-s.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *AggregationService) aggregateAndStore(ctx context.Context) error {
	// TODO: Implement aggregation logic
	// 1. Get all prices from last minute from Redis
	// 2. Calculate avg, min, max for each symbol/exchange
	// 3. Store in PostgreSQL
	// 4. Clean old data from Redis
	s.logger.Info("aggregating data")
	return nil
}
