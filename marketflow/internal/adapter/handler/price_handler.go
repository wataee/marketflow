package handler

import (
	"encoding/json"
	"log/slog"
	"marketflow/internal/application/usecase"
	"net/http"
	"strings"
	"time"
)

type PriceHandler struct {
	useCase *usecase.PriceUseCase
	logger  *slog.Logger
}

func NewPriceHandler(useCase *usecase.PriceUseCase, logger *slog.Logger) *PriceHandler {
	return &PriceHandler{
		useCase: useCase,
		logger:  logger,
	}
}

func (h *PriceHandler) GetLatestPrice(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/prices/latest/")
	parts := strings.Split(path, "/")

	var symbol, exchange string

	switch len(parts) {
	case 1:
		symbol = parts[0]
		exchange = ""
	case 2:
		exchange = parts[0]
		symbol = parts[1]
	default:
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	price, err := h.useCase.GetLatestPrice(r.Context(), symbol, exchange)
	if err != nil {
		h.logger.Error("failed to get latest price", "error", err, "symbol", symbol, "exchange", exchange)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if price == nil {
		http.Error(w, "no data found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(price)
}

func (h *PriceHandler) GetHighestPrice(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/prices/highest/")
	parts := strings.Split(path, "/")

	var symbol, exchange string
	period := parsePeriod(r, 5*time.Minute)

	switch len(parts) {
	case 1:
		symbol = parts[0]
		exchange = ""
	case 2:
		exchange = parts[0]
		symbol = parts[1]
	default:
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	result, err := h.useCase.GetHighestPrice(r.Context(), symbol, exchange, period)
	if err != nil {
		h.logger.Error("failed to get highest price", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "no data found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *PriceHandler) GetLowestPrice(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/prices/lowest/")
	parts := strings.Split(path, "/")

	var symbol, exchange string
	period := parsePeriod(r, 5*time.Minute)

	switch len(parts) {
	case 1:
		symbol = parts[0]
		exchange = ""
	case 2:
		exchange = parts[0]
		symbol = parts[1]
	default:
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	result, err := h.useCase.GetLowestPrice(r.Context(), symbol, exchange, period)
	if err != nil {
		h.logger.Error("failed to get lowest price", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "no data found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *PriceHandler) GetAveragePrice(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/prices/average/")
	parts := strings.Split(path, "/")

	var symbol, exchange string
	period := parsePeriod(r, 5*time.Minute)

	switch len(parts) {
	case 1:
		symbol = parts[0]
		exchange = ""
	case 2:
		exchange = parts[0]
		symbol = parts[1]
	default:
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	result, err := h.useCase.GetAveragePrice(r.Context(), symbol, exchange, period)
	if err != nil {
		h.logger.Error("failed to get average price", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol":   symbol,
		"exchange": exchange,
		"period":   period.String(),
		"average":  result,
	})
}

func parsePeriod(r *http.Request, defaultPeriod time.Duration) time.Duration {
	periodStr := r.URL.Query().Get("period")
	if periodStr == "" {
		return defaultPeriod
	}

	duration, err := time.ParseDuration(periodStr)
	if err != nil {
		return defaultPeriod
	}

	return duration
}