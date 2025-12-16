package generator

import (
	"context"
	"log/slog"
	"math/rand"
	"marketflow/internal/domain/model"
	"marketflow/internal/domain/port"
	"time"
)

type TestGenerator struct {
	name    string
	pairs   []string
	log     *slog.Logger
	cancel  context.CancelFunc
}

func NewTestGenerator(name string, pairs []string, log *slog.Logger) port.ExchangePort {
	return &TestGenerator{name: name, pairs: pairs, log: log}
}

func (t *TestGenerator) Name() string { return t.name }

func (t *TestGenerator) Connect(ctx context.Context) error {
	// nothing to do
	return nil
}

func (t *TestGenerator) Subscribe(symbols []string) error {
	return nil
}

func (t *TestGenerator) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	out := make(chan model.PriceUpdate)
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go func() {
		defer close(out)
		defer close(errCh)
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, pair := range t.pairs {
					price := r.Float64()*100 + 1 // случайная цена
					out <- model.PriceUpdate{
						Symbol:    pair,
						Exchange:  t.name,
						Price:     price,
						Timestamp: time.Now(),
					}
				}
			}
		}
	}()
	return out, errCh
}

func (t *TestGenerator) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}
