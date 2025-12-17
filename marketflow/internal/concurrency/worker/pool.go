package worker

import (
	"context"
	"log/slog"
	"sync"

	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
)

// Pool выполняет обработку PriceUpdate'ов.
// Обработка: запись latest -> добавление в окно (sorted set).
// При ошибке записи в Redis — fallback: записать одиночную агрегированную запись в Postgres.
type Pool struct {
	workers int
	cache   port.CachePort
	storage port.StoragePort
	logger  *slog.Logger
}

// NewPool создаёт новый пул воркеров.
func NewPool(workers int, cache port.CachePort, storage port.StoragePort, logger *slog.Logger) *Pool {
	if workers <= 0 {
		workers = 1
	}
	return &Pool{
		workers: workers,
		cache:   cache,
		storage: storage,
		logger:  logger,
	}
}

// Start запускает пул воркеров, читает из in и возвращает канал processed,
// в который помещаются элементы после обработки (чтобы не терять их в тестах/логе).
// processed закрывается, когда все воркеры завершат работу.
func (p *Pool) Start(ctx context.Context, in <-chan model.PriceUpdate) <-chan model.PriceUpdate {
	out := make(chan model.PriceUpdate)
	var wg sync.WaitGroup

	// Общий входной канал; создаём внутренний канал для работников, чтобы можно было
	// корректно закрыть их когда все данные прочитаны.
	workCh := make(chan model.PriceUpdate)

	// Горутина, которая копирует из in в workCh и закрывает workCh при завершении.
	go func() {
		defer close(workCh)
		for v := range in {
			select {
			case <-ctx.Done():
				return
			case workCh <- v:
			}
		}
	}()

	// Запускаем воркеры
	wg.Add(p.workers)
	for i := 0; i < p.workers; i++ {
		go func(id int) {
			defer wg.Done()
			p.workerLoop(ctx, id, workCh, out)
		}(i)
	}

	// Закрываем выходной канал когда все воркеры завершили
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func (p *Pool) workerLoop(ctx context.Context, id int, in <-chan model.PriceUpdate, out chan<- model.PriceUpdate) {
	for {
		select {
		case <-ctx.Done():
			return
		case pu, ok := <-in:
			if !ok {
				return
			}
			// Обрабатываем цену
			p.processOne(ctx, id, pu)

			// Отправляем в processed канал (не блокируем навсегда)
			select {
			case <-ctx.Done():
				return
			case out <- pu:
			}
		}
	}
}

func (p *Pool) processOne(ctx context.Context, id int, pu model.PriceUpdate) {
	// 1) Сохраняем latest в Redis
	if err := p.cache.SetLatestPrice(ctx, pu); err != nil {
		p.logger.Error("worker: SetLatestPrice failed, performing fallback to storage", "worker", id, "exchange", pu.Exchange, "symbol", pu.Symbol, "err", err)
		// fallback: записать в Postgres одиночную агрегированную запись
		p.fallbackWrite(ctx, pu)
		return
	}

	// 2) Добавляем в window (sorted set)
	if err := p.cache.AddPriceToWindow(ctx, pu); err != nil {
		// Если не получилось добавить в окно — логируем и пробуем fallback
		p.logger.Error("worker: AddPriceToWindow failed, performing fallback to storage", "worker", id, "exchange", pu.Exchange, "symbol", pu.Symbol, "err", err)
		p.fallbackWrite(ctx, pu)
		return
	}

	// Успешная обработка — можно логировать мелкоуровневую информацию (debug)
	p.logger.Debug("worker: processed price", "worker", id, "exchange", pu.Exchange, "symbol", pu.Symbol, "price", pu.Price)
}

// fallbackWrite записывает одно значение в Postgres как агрегат (avg=min=max=price).
func (p *Pool) fallbackWrite(ctx context.Context, pu model.PriceUpdate) {
	ap := model.AggregatedPrice{
		PairName:     pu.Symbol,
		Exchange:     pu.Exchange,
		Timestamp:    pu.Timestamp,
		AveragePrice: pu.Price,
		MinPrice:     pu.Price,
		MaxPrice:     pu.Price,
	}

	// Сохраняем в одном батче
	if err := p.storage.SaveAggregatedPrices(ctx, []model.AggregatedPrice{ap}); err != nil {
		p.logger.Error("worker: fallback SaveAggregatedPrices failed", "exchange", pu.Exchange, "symbol", pu.Symbol, "err", err)
	}
}
