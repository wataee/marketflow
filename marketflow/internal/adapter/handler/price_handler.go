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
	log     *slog.Logger
}

func NewPriceHandler(uc *usecase.PriceUseCase, log *slog.Logger) *PriceHandler {
	return &PriceHandler{
		useCase: uc,
		log:     log,
	}
}

// /prices/latest/{symbol} или /prices/latest/{exchange}/{symbol}
func (h *PriceHandler) GetLatestPrice(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/prices/latest/"), "/")
	var symbol, exchange string
	if len(parts) == 1 {
		symbol = parts[0]
	} else if len(parts) == 2 {
		exchange = parts[0]
		symbol = parts[1]
	} else {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	price, err := h.useCase.GetLatestPrice(r.Context(), symbol, exchange)
	if err != nil {
		h.log.Error("GetLatestPrice failed", "error", err)
		http.Error(w, "failed to get price", http.StatusInternalServerError)
		return
	}
	if price == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	writeJSON(w, price)
}

// /prices/highest/{symbol}?[exchange=&period=]
func (h *PriceHandler) GetHighestPrice(w http.ResponseWriter, r *http.Request) {
	h.handleAggregate(w, r, "highest")
}

// /prices/lowest/{symbol}?[exchange=&period=]
func (h *PriceHandler) GetLowestPrice(w http.ResponseWriter, r *http.Request) {
	h.handleAggregate(w, r, "lowest")
}

// /prices/average/{symbol}?[exchange=&period=]
func (h *PriceHandler) GetAveragePrice(w http.ResponseWriter, r *http.Request) {
	h.handleAggregate(w, r, "average")
}

func (h *PriceHandler) handleAggregate(w http.ResponseWriter, r *http.Request, typ string) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/prices/"+typ+"/"), "/")
	if len(pathParts) == 0 {
		http.Error(w, "symbol missing", http.StatusBadRequest)
		return
	}
	var symbol, exchange string
	if len(pathParts) == 1 {
		symbol = pathParts[0]
	} else if len(pathParts) == 2 {
		exchange = pathParts[0]
		symbol = pathParts[1]
	}

	// period
	periodStr := r.URL.Query().Get("period")
	period := time.Minute // default
	if periodStr != "" {
		d, err := time.ParseDuration(periodStr)
		if err == nil {
			period = d
		}
	}

	var result interface{}
	var err error

	switch typ {
	case "highest":
		result, err = h.useCase.GetHighestPrice(r.Context(), symbol, exchange, period)
	case "lowest":
		result, err = h.useCase.GetLowestPrice(r.Context(), symbol, exchange, period)
	case "average":
		result, err = h.useCase.GetAveragePrice(r.Context(), symbol, exchange, period)
	default:
		http.Error(w, "unknown type", http.StatusBadRequest)
		return
	}

	if err != nil {
		h.log.Error("aggregate failed", "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	if result == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
