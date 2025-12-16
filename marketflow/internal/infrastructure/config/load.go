package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Server.ReadTimeout, err = time.ParseDuration(cfg.Server.ReadTimeoutStr); err != nil {
		return nil, err
	}
	if cfg.Server.WriteTimeout, err = time.ParseDuration(cfg.Server.WriteTimeoutStr); err != nil {
		return nil, err
	}
	if cfg.Server.ShutdownTimeout, err = time.ParseDuration(cfg.Server.ShutdownTimeoutStr); err != nil {
		return nil, err
	}

	if cfg.PostgreSQL.ConnMaxLifetime, err = time.ParseDuration(cfg.PostgreSQL.ConnMaxLifetimeStr); err != nil {
		return nil, err
	}

	if cfg.Workers.BatchTimeout, err = time.ParseDuration(cfg.Workers.BatchTimeoutStr); err != nil {
		return nil, err
	}

	if cfg.DataRetention.RedisTTL, err = time.ParseDuration(cfg.DataRetention.RedisTTLStr); err != nil {
		return nil, err
	}
	if cfg.DataRetention.AggregationInterval, err = time.ParseDuration(cfg.DataRetention.AggregationIntervalStr); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.PostgreSQL.Host, c.PostgreSQL.Port, c.PostgreSQL.User,
		c.PostgreSQL.Password, c.PostgreSQL.Database, c.PostgreSQL.SSLMode,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}
