package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port            int           `yaml:"port"`
		ReadTimeout     time.Duration `yaml:"read_timeout"`
		WriteTimeout    time.Duration `yaml:"write_timeout"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"server"`

	PostgreSQL struct {
		Host            string        `yaml:"host"`
		Port            int           `yaml:"port"`
		User            string        `yaml:"user"`
		Password        string        `yaml:"password"`
		Database        string        `yaml:"database"`
		SSLMode         string        `yaml:"sslmode"`
		MaxOpenConns    int           `yaml:"max_open_conns"`
		MaxIdleConns    int           `yaml:"max_idle_conns"`
		ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	} `yaml:"postgresql"`

	Redis struct {
		Host         string `yaml:"host"`
		Port         int    `yaml:"port"`
		Password     string `yaml:"password"`
		DB           int    `yaml:"db"`
		PoolSize     int    `yaml:"pool_size"`
		MinIdleConns int    `yaml:"min_idle_conns"`
	} `yaml:"redis"`

	Exchanges []struct {
		Name    string `yaml:"name"`
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
		Enabled bool   `yaml:"enabled"`
	} `yaml:"exchanges"`

	TestGenerator struct {
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
		Enabled bool   `yaml:"enabled"`
	} `yaml:"test_generator"`

	TradingPairs []string `yaml:"trading_pairs"`

	Workers struct {
		PerExchange  int           `yaml:"per_exchange"`
		BatchSize    int           `yaml:"batch_size"`
		BatchTimeout time.Duration `yaml:"batch_timeout"`
	} `yaml:"workers"`

	DataRetention struct {
		RedisTTL            time.Duration `yaml:"redis_ttl"`
		AggregationInterval time.Duration `yaml:"aggregation_interval"`
	} `yaml:"data_retention"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.PostgreSQL.Host,
		c.PostgreSQL.Port,
		c.PostgreSQL.User,
		c.PostgreSQL.Password,
		c.PostgreSQL.Database,
		c.PostgreSQL.SSLMode,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}
