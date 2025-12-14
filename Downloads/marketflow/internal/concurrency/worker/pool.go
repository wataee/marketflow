package worker

import (
	"context"
	"log/slog"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"sync"
)

type Pool struct {
	workers int
	cache   port.CachePort
	storage port.StoragePort
	logger  *slog.Logger
}

func NewPool(workers int, cache port.CachePort, storage port.StoragePort, logger *slog.Logger) *Pool {
	return &Pool{
		workers: workers,
		cache:   cache,
		storage: storage,
		logger:  logger,
	}
}

func (p *Pool) Start(ctx context.Context, in <-chan model.PriceUpdate) <-chan model.PriceUpdate {
	out := make(chan model.PriceUpdate)
	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(ctx, workerID, in, out)
		}(i)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func (p *Pool) worker(ctx context.Context, id int, in <-chan model.PriceUpdate, out chan<- model.PriceUpdate) {
	for {
		select {
		case <-ctx.Done():
			return
		case price, ok := <-in:
			if !ok {
				return
			}

			// Обработка: сохранение в Redis
			if err := p.cache.SetLatestPrice(ctx, price); err != nil {
				p.logger.Error("failed to cache price",
					"worker", id,
					"symbol", price.Symbol,
					"exchange", price.Exchange,
					"error", err)
			}

			// Добавление в окно для агрегации
			if err := p.cache.AddPriceToWindow(ctx, price); err != nil {
				p.logger.Error("failed to add to window",
					"worker", id,
					"error", err)
			}

			out <- price
		}
	}
}
