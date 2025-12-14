package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"marketflow/internal/domain/model"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisAdapter struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisAdapter(addr, password string, db int, ttl time.Duration) (*RedisAdapter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisAdapter{
		client: client,
		ttl:    ttl,
	}, nil
}

func (a *RedisAdapter) SetLatestPrice(ctx context.Context, price model.PriceUpdate) error {
	key := fmt.Sprintf("latest:%s:%s", price.Exchange, price.Symbol)
	data, err := json.Marshal(price)
	if err != nil {
		return err
	}
	return a.client.Set(ctx, key, data, a.ttl).Err()
}

func (a *RedisAdapter) GetLatestPrice(ctx context.Context, symbol, exchange string) (*model.LatestPrice, error) {
	key := fmt.Sprintf("latest:%s:%s", exchange, symbol)
	data, err := a.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var price model.PriceUpdate
	if err := json.Unmarshal(data, &price); err != nil {
		return nil, err
	}

	return &model.LatestPrice{
		Symbol:    price.Symbol,
		Exchange:  price.Exchange,
		Price:     price.Price,
		Timestamp: price.Timestamp,
	}, nil
}

func (a *RedisAdapter) AddPriceToWindow(ctx context.Context, price model.PriceUpdate) error {
	// TODO: Use sorted set for time-based window
	return nil
}

func (a *RedisAdapter) GetPricesInWindow(ctx context.Context, symbol, exchange string) ([]model.PriceUpdate, error) {
	// TODO: Implement
	return nil, nil
}

func (a *RedisAdapter) DeleteOldPrices(ctx context.Context, before time.Time) error {
	// TODO: Implement cleanup
	return nil
}

func (a *RedisAdapter) Close() error {
	return a.client.Close()
}
