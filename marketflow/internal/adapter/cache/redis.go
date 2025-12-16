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

func (a *RedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

func (a *RedisAdapter) SetLatestPrice(ctx context.Context, price model.PriceUpdate) error {
	key := fmt.Sprintf("latest:%s:%s", price.Exchange, price.Symbol)
	data, err := json.Marshal(price)
	if err != nil {
		return fmt.Errorf("failed to marshal price: %w", err)
	}
	
	if err := a.client.Set(ctx, key, data, a.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set latest price in redis: %w", err)
	}
	return nil
}

func (a *RedisAdapter) GetLatestPrice(ctx context.Context, symbol, exchange string) (*model.LatestPrice, error) {
	key := fmt.Sprintf("latest:%s:%s", exchange, symbol)
	data, err := a.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest price from redis: %w", err)
	}

	var price model.PriceUpdate
	if err := json.Unmarshal(data, &price); err != nil {
		return nil, fmt.Errorf("failed to unmarshal price: %w", err)
	}

	return &model.LatestPrice{
		Symbol:    price.Symbol,
		Exchange:  price.Exchange,
		Price:     price.Price,
		Timestamp: price.Timestamp,
	}, nil
}

// AddPriceToWindow добавляет цену в sorted set для window (score = unix seconds)
func (a *RedisAdapter) AddPriceToWindow(ctx context.Context, price model.PriceUpdate) error {
	key := fmt.Sprintf("window:%s:%s", price.Exchange, price.Symbol)
	data, err := json.Marshal(price)
	if err != nil {
		return fmt.Errorf("failed to marshal price for window: %w", err)
	}
	
	z := redis.Z{
		Score:  float64(price.Timestamp.Unix()),
		Member: data,
	}

	if err := a.client.ZAdd(ctx, key, z).Err(); err != nil {
		return fmt.Errorf("failed to add price to window: %w", err)
	}

	_ = a.client.Expire(ctx, key, a.ttl*2).Err()
	return nil
}

// GetPricesInWindow возвращает цены за последний TTL для symbol/exchange
func (a *RedisAdapter) GetPricesInWindow(ctx context.Context, symbol, exchange string) ([]model.PriceUpdate, error) {
	now := time.Now()
	minTs := now.Add(-a.ttl).Unix()
	maxTs := now.Unix()

	var keys []string
	if exchange != "" {
		keys = []string{fmt.Sprintf("window:%s:%s", exchange, symbol)}
	} else {
		pattern := fmt.Sprintf("window:*:%s", symbol)
		iter := a.client.Scan(ctx, 0, pattern, 0).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("failed to scan redis keys: %w", err)
		}
	}

	var out []model.PriceUpdate

	for _, key := range keys {
		minStr := strconv.FormatInt(minTs, 10)
		maxStr := strconv.FormatInt(maxTs, 10)
		
		results, err := a.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min:    minStr,
			Max:    maxStr,
			Offset: 0,
			Count:  0,
		}).Result()
		
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("failed to get prices from window %s: %w", key, err)
		}
		
		for _, item := range results {
			var pu model.PriceUpdate
			if err := json.Unmarshal([]byte(item), &pu); err != nil {
				return nil, fmt.Errorf("failed to unmarshal price from redis key %s: %w", key, err)
			}
			out = append(out, pu)
		}
	}

	return out, nil
}

// DeleteOldPrices удаляет записи старше before по всем window ключам
func (a *RedisAdapter) DeleteOldPrices(ctx context.Context, before time.Time) error {
	max := strconv.FormatInt(before.Unix(), 10)
	pattern := "window:*:*"
	
	iter := a.client.Scan(ctx, 0, pattern, 0).Iterator()
	deletedCount := 0
	
	for iter.Next(ctx) {
		key := iter.Val()
		deleted, err := a.client.ZRemRangeByScore(ctx, key, "-inf", max).Result()
		if err != nil {
			return fmt.Errorf("failed to delete old prices from %s: %w", key, err)
		}
		deletedCount += int(deleted)
	}
	
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to iterate redis keys: %w", err)
	}
	
	if deletedCount > 0 {
		// Can add logging here if needed
	}
	
	return nil
}

func (a *RedisAdapter) Close() error {
	return a.client.Close()
}