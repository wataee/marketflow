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

// AggregationService агрегирует данные из Redis и сохраняет в Postgres пакетно.
type AggregationService struct {
	cache        port.CachePort
	storage      port.StoragePort
	logger       *slog.Logger
	ticker       *time.Ticker
	done         chan struct{}
	tradingPairs []string
	retention    time.Duration
	mu           sync.RWMutex
}

// NewAggregationService создаёт сервис. tradingPairs можно задавать либо через SetTradingPairs,
// либо через переменную окружения TRADING_PAIRS (comma-separated). retention будет установлен в Start.
func NewAggregationService(cache port.CachePort, storage port.StoragePort, logger *slog.Logger) *AggregationService {
	s := &AggregationService{
		cache:   cache,
		storage: storage,
		logger:  logger,
		done:    make(chan struct{}),
	}

	// Попытка получить пары из ENV как fallback: "BTCUSDT,DOGEUSDT,TONUSDT,SOLUSDT,ETHUSDT"
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

// SetTradingPairs задаёт список торговых пар для агрегации (предпочтительный способ).
func (s *AggregationService) SetTradingPairs(pairs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tradingPairs = append([]string{}, pairs...)
}

// Start запускает цикл агрегации с интервалом interval.
// retention используется для очистки старых значений в Redis (по умолчанию = interval).
func (s *AggregationService) Start(ctx context.Context, interval time.Duration) {
	s.mu.Lock()
	if interval <= 0 {
		interval = time.Minute
	}
	s.retention = interval
	s.ticker = time.NewTicker(interval)
	s.mu.Unlock()

	go s.aggregateLoop(ctx)
}

func (s *AggregationService) Stop() {
	s.mu.Lock()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
	s.mu.Unlock()
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

// aggregateAndStore берет данные из Redis по каждой паре, считает min/max/avg и сохраняет пакетно в Postgres.
// Предположения:
// - cache.GetPricesInWindow(ctx, symbol, exchange) поддерживает exchange == "" (все биржи) или конкретный exchange.
// - storage.SaveAggregatedPrices выполняет пакетную вставку.
func (s *AggregationService) aggregateAndStore(ctx context.Context) error {
	s.mu.RLock()
	pairs := append([]string{}, s.tradingPairs...)
	retention := s.retention
	s.mu.RUnlock()

	if len(pairs) == 0 {
		return errors.New("no trading pairs configured for aggregation")
	}

	var batch []model.AggregatedPrice

	for _, pair := range pairs {
		// пытаемся получить данные без указания exchange (все биржи). При необходимости можно менять.
		prices, err := s.cache.GetPricesInWindow(ctx, pair, "")
		if err != nil {
			s.logger.Error("failed to get prices from cache", "pair", pair, "error", err)
			continue
		}
		if len(prices) == 0 {
			continue
		}

		avg, mn, mx := computeStats(prices)

		ap := model.AggregatedPrice{
			PairName:     pair,
			Exchange:     "", // если хотите хранить по-эксченджево, нужно менять cache API / Call per-exchange
			Timestamp:    time.Now().UTC(),
			AveragePrice: avg,
			MinPrice:     mn,
			MaxPrice:     mx,
		}
		batch = append(batch, ap)
	}

	if len(batch) == 0 {
		s.logger.Info("aggregation: nothing to store")
		// Очистим старые данные в Redis
		_ = s.cache.DeleteOldPrices(ctx, time.Now().Add(-retention))
		return nil
	}

	// Сохраняем пакетно
	if err := s.storage.SaveAggregatedPrices(ctx, batch); err != nil {
		return err
	}

	// Очищаем старые данные (время берём как now - retention)
	if err := s.cache.DeleteOldPrices(ctx, time.Now().Add(-retention)); err != nil {
		// не фатальная ошибка — логируем
		s.logger.Error("failed to delete old prices from cache", "error", err)
	}

	s.logger.Info("aggregation: saved batch", "count", len(batch))
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
