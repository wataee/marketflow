package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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

	// Применяем переменные окружения (переопределяют значения из файла)
	applyEnvOverrides(&cfg)

	// Парсим duration'ы
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

func applyEnvOverrides(cfg *Config) {
	// PostgreSQL
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		cfg.PostgreSQL.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.PostgreSQL.Port = port
		}
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		cfg.PostgreSQL.User = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		cfg.PostgreSQL.Password = v
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		cfg.PostgreSQL.Database = v
	}

	// Redis
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = port
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	// Exchanges
	if v := os.Getenv("EXCHANGE1_HOST"); v != "" && len(cfg.Exchanges) > 0 {
		cfg.Exchanges[0].Host = v
	}
	if v := os.Getenv("EXCHANGE2_HOST"); v != "" && len(cfg.Exchanges) > 1 {
		cfg.Exchanges[1].Host = v
	}
	if v := os.Getenv("EXCHANGE3_HOST"); v != "" && len(cfg.Exchanges) > 2 {
		cfg.Exchanges[2].Host = v
	}

	// Server
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
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
