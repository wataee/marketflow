package handler

import (
	"encoding/json"
	"log/slog"
	"marketflow/internal/application/usecase"
	"net/http"
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
	// TODO: Extract symbol and exchange from URL
	// TODO: Call useCase.GetLatestPrice
	// TODO: Return JSON response
	h.logger.Info("get latest price request")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *PriceHandler) GetHighestPrice(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	h.logger.Info("get highest price request")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *PriceHandler) GetLowestPrice(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	h.logger.Info("get lowest price request")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *PriceHandler) GetAveragePrice(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	h.logger.Info("get average price request")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func parseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}
