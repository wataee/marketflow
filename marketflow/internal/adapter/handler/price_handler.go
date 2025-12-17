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

func (h *PriceHandler) GetLatestPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("invalid method for GetLatestPrice", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/prices/latest/"), "/")
	var symbol, exchange string
	
	if len(parts) == 1 {
		symbol = parts[0]
	} else if len(parts) == 2 {
		exchange = parts[0]
		symbol = parts[1]
	} else {
		h.log.Warn("invalid path format", "path", r.URL.Path)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	h.log.Info("getting latest price", "symbol", symbol, "exchange", exchange)

	price, err := h.useCase.GetLatestPrice(r.Context(), symbol, exchange)
	if err != nil {
		h.log.Error("GetLatestPrice failed", "symbol", symbol, "exchange", exchange, "error", err)
		http.Error(w, "failed to get price", http.StatusInternalServerError)
		return
	}
	
	if price == nil {
		h.log.Info("price not found", "symbol", symbol, "exchange", exchange)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	h.log.Info("latest price retrieved", "symbol", symbol, "exchange", exchange, "price", price.Price)
	writeJSON(w, price)
}

func (h *PriceHandler) GetHighestPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("invalid method for GetHighestPrice", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.handleAggregate(w, r, "highest")
}

func (h *PriceHandler) GetLowestPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("invalid method for GetLowestPrice", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.handleAggregate(w, r, "lowest")
}

func (h *PriceHandler) GetAveragePrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("invalid method for GetAveragePrice", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.handleAggregate(w, r, "average")
}

func (h *PriceHandler) handleAggregate(w http.ResponseWriter, r *http.Request, typ string) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/prices/"+typ+"/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		h.log.Warn("symbol missing in path", "type", typ, "path", r.URL.Path)
		http.Error(w, "symbol missing", http.StatusBadRequest)
		return
	}
	
	var symbol, exchange string
	if len(pathParts) == 1 {
		symbol = pathParts[0]
	} else if len(pathParts) == 2 {
		exchange = pathParts[0]
		symbol = pathParts[1]
	} else {
		h.log.Warn("invalid path format", "type", typ, "path", r.URL.Path)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	periodStr := r.URL.Query().Get("period")
	period := time.Minute
	if periodStr != "" {
		d, err := time.ParseDuration(periodStr)
		if err != nil {
			h.log.Warn("invalid period format", "period", periodStr, "error", err)
			http.Error(w, "invalid period format", http.StatusBadRequest)
			return
		}
		period = d
	}

	h.log.Info("handling aggregate request",
		"type", typ,
		"symbol", symbol,
		"exchange", exchange,
		"period", period)

	var result interface{}
	var err error

	switch typ {
	case "highest":
		result, err = h.useCase.GetHighestPrice(r.Context(), symbol, exchange, period)
	case "lowest":
		result, err = h.useCase.GetLowestPrice(r.Context(), symbol, exchange, period)
	case "average":
		avgVal, avgErr := h.useCase.GetAveragePrice(r.Context(), symbol, exchange, period)
		err = avgErr
		if err == nil {
			result = map[string]interface{}{
				"symbol":   symbol,
				"exchange": exchange,
				"period":   period.String(),
				"average":  avgVal,
			}
		}
	default:
		h.log.Error("unknown aggregate type", "type", typ)
		http.Error(w, "unknown type", http.StatusBadRequest)
		return
	}

	if err != nil {
		h.log.Error("aggregate operation failed",
			"type", typ,
			"symbol", symbol,
			"exchange", exchange,
			"error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	
	if result == nil {
		h.log.Info("aggregate result not found",
			"type", typ,
			"symbol", symbol,
			"exchange", exchange)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	h.log.Info("aggregate result retrieved",
		"type", typ,
		"symbol", symbol,
		"exchange", exchange)
	writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}
	w.Write(payload)
}