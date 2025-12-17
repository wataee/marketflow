package port

import (
	"context"

	"marketflow/internal/domain/model"
)

// ExchangePort определяет интерфейс для подключения к биржам
type ExchangePort interface {
	Connect(ctx context.Context) error
	Subscribe(symbols []string) error
	ReadPrices(ctx context.Context) (<-chan model.PriceUpdate, <-chan error)
	Close() error
	Name() string
}
