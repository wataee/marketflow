package config

import "time"

type Config struct {
	Server struct {
		Port               int
		ReadTimeoutStr     string `json:"read_timeout"`
		WriteTimeoutStr    string `json:"write_timeout"`
		ShutdownTimeoutStr string `json:"shutdown_timeout"`
		ReadTimeout        time.Duration
		WriteTimeout       time.Duration
		ShutdownTimeout    time.Duration
	} `json:"server"`

	PostgreSQL struct {
		Host               string
		Port               int
		User               string
		Password           string
		Database           string
		SSLMode            string
		MaxOpenConns       int    `json:"max_open_conns"`
		MaxIdleConns       int    `json:"max_idle_conns"`
		ConnMaxLifetimeStr string `json:"conn_max_lifetime"`
		ConnMaxLifetime    time.Duration
	} `json:"postgresql"`

	Redis struct {
		Host         string
		Port         int
		Password     string
		DB           int
		PoolSize     int `json:"pool_size"`
		MinIdleConns int `json:"min_idle_conns"`
	} `json:"redis"`

	Exchanges []struct {
		Name    string
		Host    string
		Port    int
		Enabled bool
	} `json:"exchanges"`

	TestGenerator struct {
		Host    string
		Port    int
		Enabled bool
	} `json:"test_generator"`

	TradingPairs []string `json:"trading_pairs"`

	Workers struct {
		PerExchange     int    `json:"per_exchange"`
		BatchSize       int    `json:"batch_size"`
		BatchTimeoutStr string `json:"batch_timeout"`
		BatchTimeout    time.Duration
	} `json:"workers"`

	DataRetention struct {
		RedisTTLStr            string `json:"redis_ttl"`
		AggregationIntervalStr string `json:"aggregation_interval"`
		RedisTTL               time.Duration
		AggregationInterval    time.Duration
	} `json:"data_retention"`

	Logging struct {
		Level  string `json:"level"`
		Format string `json:"format"`
	} `json:"logging"`
}
