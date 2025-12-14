package generator

import (
	"context"
	"fmt"
	"log/slog"
	"marketflow/internal/domain/model"
	"math/rand"
	"time"
)

type TestGenerator struct {
	name    string
	symbols []string
	logger  *slog.Logger
}

func NewTestGenerator(name string, symbols []string, logger *slog.Logger) *TestGenerator {
	return &TestGenerator{
		name:    name,
		symbols: symbols,
		logger:  logger,
	}
}

func (g *TestGenerator) Connect(ctx context.Context) error {
	g.logger.Info("test generator started", "name", g.name)
	return nil
}

func (g *TestGenerator) Subscribe(symbols []string) error {
	return nil
}

func (g *TestGenerator) ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error) {
	priceCh := make(chan model.PriceUpdate)
	errCh := make(chan error, 1)

	go func() {
		defer close(priceCh)
		defer close(errCh)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		basePrices := map[string]float64{
			"BTCUSDT":  50000.0,
			"ETHUSDT":  3000.0,
			"DOGEUSDT": 0.1,
			"TONUSDT":  5.0,
			"SOLUSDT":  100.0,
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, symbol := range g.symbols {
					basePrice := basePrices[symbol]
					variation := (rand.Float64() - 0.5) * 0.01 * basePrice
					price := basePrice + variation

					priceCh <- model.PriceUpdate{
						Symbol:    symbol,
						Exchange:  g.name,
						Price:     price,
						Timestamp: time.Now(),
					}
				}
			}
		}
	}()

	return priceCh, errCh
}

func (g *TestGenerator) Close() error {
	return nil
}

func (g *TestGenerator) Name() string {
	return fmt.Sprintf("test-%s", g.name)
}
