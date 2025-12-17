package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
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

// NewAggregationService создаёт сервис агрегации.
// Он читает TRADING_PAIRS из окружения, если переменная задана.
func NewAggregationService(cache port.CachePort, storage port.StoragePort, logger *slog.Logger) *AggregationService {
	s := &AggregationService{
		cache:   cache,
		storage: storage,
		logger:  logger,
		done:    make(chan struct{}),
		// По умолчанию ретеншн 60s (будет переопределён в Start)
		retention: time.Minute,
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

// Start запускает цикл агрегации с указанным интервалом.
// Если interval <= 0, используется 1 минута.
func (s *AggregationService) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	s.mu.Lock()
	// остановим предыдущий тикер если был
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.retention = interval
	s.ticker = time.NewTicker(interval)
	s.mu.Unlock()

	s.logger.Info("aggregation service starting", "interval", interval.String())

	go s.aggregateLoop(ctx)
}

// Stop корректно останавливает сервис и запускает финальную агрегацию.
func (s *AggregationService) Stop() {
	s.mu.Lock()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	// закрываем done только один раз
	select {
	case <-s.done:
		// уже закрыт
	default:
		close(s.done)
	}
	s.mu.Unlock()
	s.logger.Info("aggregation service stopped")
}

func (s *AggregationService) aggregateLoop(ctx context.Context) {
	s.logger.Info("aggregation loop started")
	// Гарантируем выполнение финальной агрегации при выходе
	defer func() {
		s.logger.Info("running final aggregation before exit")
		// используем background context чтобы завершить финальную запись
		_ = s.aggregateAndStore(context.Background())
	}()

	for {
		s.mu.RLock()
		tick := s.ticker
		s.mu.RUnlock()

		// Если тикер ещё не установлен (маловероятно), подождём немного
		if tick == nil {
			select {
			case <-time.After(time.Second):
				// повторить
				continue
			case <-s.done:
				return
			}
		}

		select {
		case <-tick.C:
			s.logger.Info("starting aggregation cycle")
			start := time.Now()
			if err := s.aggregateAndStore(ctx); err != nil {
				s.logger.Error("aggregation failed", "error", err, "duration", time.Since(start))
			} else {
				s.logger.Info("aggregation cycle completed", "duration", time.Since(start))
			}
		case <-s.done:
			s.logger.Info("aggregation loop stopping by done channel")
			return
		case <-ctx.Done():
			s.logger.Info("aggregation loop cancelled by context")
			return
		}
	}
}

func (s *AggregationService) aggregateAndStore(ctx context.Context) error {
	// Снимем конфигурацию под read lock
	s.mu.RLock()
	pairs := append([]string{}, s.tradingPairs...)
	exchanges := append([]string{}, s.exchanges...)
	retention := s.retention
	s.mu.RUnlock()

	if len(pairs) == 0 {
		s.logger.Warn("no trading pairs configured for aggregation")
		return errors.New("no trading pairs configured for aggregation")
	}

	s.logger.Debug("aggregating data", "pairs", len(pairs), "exchanges", len(exchanges))

	var batch []model.AggregatedPrice

	// Если заданы явные exchanges — агрегируем для каждой пары и каждой биржи
	if len(exchanges) > 0 {
		for _, exch := range exchanges {
			for _, pair := range pairs {
				prices, err := s.cache.GetPricesInWindow(ctx, pair, exch)
				if err != nil {
					s.logger.Error("failed to get prices from cache", "pair", pair, "exchange", exch, "error", err)
					continue
				}
				if len(prices) == 0 {
					s.logger.Debug("no prices in window", "pair", pair, "exchange", exch)
					continue
				}
				avg, mn, mx := computeStats(prices)
				batch = append(batch, model.AggregatedPrice{
					PairName:     pair,
					Exchange:     exch,
					Timestamp:    time.Now().UTC(),
					AveragePrice: avg,
					MinPrice:     mn,
					MaxPrice:     mx,
				})
				s.logger.Debug("aggregated", "pair", pair, "exchange", exch, "count", len(prices), "avg", avg, "min", mn, "max", mx)
			}
		}
	} else {
		// Если exchanges не заданы, просим кеш вернуть все значения для пары (включая все биржи)
		for _, pair := range pairs {
			prices, err := s.cache.GetPricesInWindow(ctx, pair, "")
			if err != nil {
				s.logger.Error("failed to get prices from cache", "pair", pair, "error", err)
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

			for exch, exPrices := range pricesByExchange {
				avg, mn, mx := computeStats(exPrices)
				batch = append(batch, model.AggregatedPrice{
					PairName:     pair,
					Exchange:     exch,
					Timestamp:    time.Now().UTC(),
					AveragePrice: avg,
					MinPrice:     mn,
					MaxPrice:     mx,
				})
				s.logger.Debug("aggregated", "pair", pair, "exchange", exch, "count", len(exPrices), "avg", avg, "min", mn, "max", mx)
			}
		}
	}

	if len(batch) == 0 {
		s.logger.Info("aggregation: nothing to store")
		// Очистка старых значений в кеше — аккуратно, игнорируем ошибку, но логируем
		if err := s.cache.DeleteOldPrices(ctx, time.Now().Add(-retention)); err != nil {
			s.logger.Error("failed to delete old prices from cache", "error", err)
		} else {
			s.logger.Debug("old prices deleted from cache")
		}
		return nil
	}

	// Сохраняем в БД батч
	s.logger.Info("saving aggregated batch", "count", len(batch))
	if err := s.storage.SaveAggregatedPrices(ctx, batch); err != nil {
		s.logger.Error("failed to save aggregated prices", "error", err, "batch_size", len(batch))
		// НЕ удаляем данные из кеша, т.к. сохранение не выполнено
		return err
	}
	s.logger.Info("aggregated batch saved successfully", "count", len(batch))

	// Удаляем старые цены из кеша только после успешного сохранения
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
