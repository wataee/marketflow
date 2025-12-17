package model

import "time"

type PriceUpdate struct {
	Symbol    string    `json:"symbol"`
	Exchange  string    `json:"exchange"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

type AggregatedPrice struct {
	PairName     string    `json:"pair_name"`
	Exchange     string    `json:"exchange"`
	Timestamp    time.Time `json:"timestamp"`
	AveragePrice float64   `json:"average_price"`
	MinPrice     float64   `json:"min_price"`
	MaxPrice     float64   `json:"max_price"`
}

type LatestPrice struct {
	Symbol    string    `json:"symbol"`
	Exchange  string    `json:"exchange"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}
