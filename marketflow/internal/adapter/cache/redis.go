package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"marketflow/internal/domain/model"
	"strconv"
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

// Ping реализует порт Ping
func (a *RedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

func (a *RedisAdapter) SetLatestPrice(ctx context.Context, price model.PriceUpdate) error {
	key := fmt.Sprintf("latest:%s:%s", price.Exchange, price.Symbol)
	data, err := json.Marshal(price)
	if err != nil {
		return err
	}
	// Сохраняем с TTL
	if err := a.client.Set(ctx, key, data, a.ttl).Err(); err != nil {
		return err
	}
	return nil
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

// AddPriceToWindow — добавляет цену в sorted set для window (score = unix ms)
// Ключ: window:{exchange}:{symbol}
func (a *RedisAdapter) AddPriceToWindow(ctx context.Context, price model.PriceUpdate) error {
	key := fmt.Sprintf("window:%s:%s", price.Exchange, price.Symbol)
	data, err := json.Marshal(price)
	if err != nil {
		return err
	}
	z := redis.Z{
		Score:  float64(price.Timestamp.Unix()),
		Member: data, // данные в формате JSON
	}

	if err := a.client.ZAdd(ctx, key, z).Err(); err != nil {
		return err
	}

	// Обновляем TTL чтобы ключ не жил вечно
	_ = a.client.Expire(ctx, key, a.ttl*2).Err()
	return nil
}

// GetPricesInWindow возвращает цены за последний TTL (a.ttl) для symbol/exchange.
// Если exchange == "" — собирает по всем exchange'ам (сканирует ключи window:*:symbol).
func (a *RedisAdapter) GetPricesInWindow(ctx context.Context, symbol, exchange string) ([]model.PriceUpdate, error) {
	now := time.Now()
	minTs := now.Add(-a.ttl).UnixMilli()
	maxTs := now.UnixMilli()

	var keys []string
	if exchange != "" {
		keys = []string{fmt.Sprintf("window:%s:%s", exchange, symbol)}
	} else {
		// scan all keys matching pattern window:*:symbol
		pattern := fmt.Sprintf("window:*:%s", symbol)
		iter := a.client.Scan(ctx, 0, pattern, 0).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if err := iter.Err(); err != nil {
			return nil, err
		}
	}

	var out []model.PriceUpdate

	for _, key := range keys {
		// ZRangeByScore with min..max (both inclusive)
		minStr := strconv.FormatInt(minTs, 10)
		maxStr := strconv.FormatInt(maxTs, 10)
		results, err := a.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min:    minStr,
			Max:    maxStr,
			Offset: 0,
			Count:  0,
		}).Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		for _, item := range results {
			var pu model.PriceUpdate
			if err := json.Unmarshal([]byte(item), &pu); err != nil {
				// если не удалось распарсить — логируем в виде ошибки возврата
				return nil, fmt.Errorf("failed to unmarshal price from redis key %s: %w", key, err)
			}
			out = append(out, pu)
		}
	}

	// Опционально: можно отсортировать по Timestamp (ascending)
	// Но большинство агрегирующих алгоритмов не требует строгой сортировки.
	return out, nil
}

// DeleteOldPrices удаляет записи старше before по всем window ключам.
func (a *RedisAdapter) DeleteOldPrices(ctx context.Context, before time.Time) error {
	max := strconv.FormatInt(before.UnixMilli(), 10)
	pattern := "window:*:*"
	iter := a.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if err := a.client.ZRemRangeByScore(ctx, key, "-inf", max).Err(); err != nil {
			return err
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	return nil
}

func (a *RedisAdapter) Close() error {
	return a.client.Close()
}
