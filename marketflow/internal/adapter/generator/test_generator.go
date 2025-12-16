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
	return &TestGenerator{
		name:  name,
		pairs: pairs,
		log:   log,
	}
}

func (t *TestGenerator) Name() string {
	return t.name
}

func (t *TestGenerator) Connect(ctx context.Context) error {
	t.log.Info("test generator connected", "name", t.name, "pairs", len(t.pairs))
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

	t.log.Info("starting test price generation", "name", t.name, "pairs", t.pairs)

	go func() {
		defer close(out)
		defer close(errCh)
		
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		
		generatedCount := 0
		
		for {
			select {
			case <-ctx.Done():
				t.log.Info("test generator stopped", "name", t.name, "total_generated", generatedCount)
				return
			case <-ticker.C:
				for _, pair := range t.pairs {
					price := r.Float64()*100 + 1
					
					out <- model.PriceUpdate{
						Symbol:    pair,
						Exchange:  t.name,
						Price:     price,
						Timestamp: time.Now(),
					}
					
					generatedCount++
				}
				
				if generatedCount%100 == 0 {
					t.log.Debug("test prices generated", "name", t.name, "count", generatedCount)
				}
			}
		}
	}()
	
	return out, errCh
}

func (t *TestGenerator) Close() error {
	t.log.Info("closing test generator", "name", t.name)
	
	if t.cancel != nil {
		t.cancel()
	}
	
	t.log.Info("test generator closed", "name", t.name)
	return nil
}