package model

import "time"

// PriceUpdate представляет обновление цены от биржи
type PriceUpdate struct {
	Symbol    string
	Exchange  string
	Price     float64
	Timestamp time.Time
}

// AggregatedPrice представляет агрегированные данные за период
type AggregatedPrice struct {
	PairName     string
	Exchange     string
	Timestamp    time.Time
	AveragePrice float64
	MinPrice     float64
	MaxPrice     float64
}

// LatestPrice представляет последнюю цену для API
type LatestPrice struct {
	Symbol    string
	Exchange  string
	Price     float64
	Timestamp time.Time
}
