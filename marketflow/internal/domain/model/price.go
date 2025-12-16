package model

import "time"

type PriceUpdate struct {
	Symbol    string
	Exchange  string
	Price     float64
	Timestamp time.Time
}

type AggregatedPrice struct {
	PairName     string
	Exchange     string
	Timestamp    time.Time
	AveragePrice float64
	MinPrice     float64
	MaxPrice     float64
}

type LatestPrice struct {
	Symbol    string
	Exchange  string
	Price     float64
	Timestamp time.Time
}
