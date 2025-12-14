DROP TABLE IF EXISTS aggregated_prices CASCADE;
DROP INDEX IF EXISTS idx_pair_exchange_timestamp;
DROP INDEX IF EXISTS idx_timestamp;
DROP INDEX IF EXISTS idx_pair_name;
DROP INDEX IF EXISTS idx_exchange;
DROP INDEX IF EXISTS idx_max_price;
DROP INDEX IF EXISTS idx_min_price;

CREATE TABLE aggregated_prices (
    id SERIAL PRIMARY KEY,
    pair_name VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    average_price DOUBLE PRECISION NOT NULL,
    min_price DOUBLE PRECISION NOT NULL,
    max_price DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT chk_average_price_positive CHECK (average_price > 0),
    CONSTRAINT chk_min_price_positive CHECK (min_price > 0),
    CONSTRAINT chk_max_price_positive CHECK (max_price > 0),
    CONSTRAINT chk_min_max_price CHECK (min_price <= max_price),
    CONSTRAINT chk_avg_in_range CHECK (average_price >= min_price AND average_price <= max_price)
);

CREATE INDEX idx_pair_exchange_timestamp ON aggregated_prices(pair_name, exchange, timestamp DESC);
CREATE INDEX idx_timestamp ON aggregated_prices(timestamp DESC);
CREATE INDEX idx_pair_name ON aggregated_prices(pair_name);
CREATE INDEX idx_exchange ON aggregated_prices(exchange);
CREATE INDEX idx_max_price ON aggregated_prices(pair_name, exchange, max_price DESC);
CREATE INDEX idx_min_price ON aggregated_prices(pair_name, exchange, min_price ASC);