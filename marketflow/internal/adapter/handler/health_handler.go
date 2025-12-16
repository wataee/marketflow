package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"marketflow/internal/domain/port"
	"net/http"
)

type HealthHandler struct {
	storage port.StoragePort
	cache   port.CachePort
	log     *slog.Logger
}

func NewHealthHandler(storage port.StoragePort, cache port.CachePort, log *slog.Logger) *HealthHandler {
	return &HealthHandler{
		storage: storage,
		cache:   cache,
		log:     log,
	}
}

type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("invalid method for health check", "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.log.Info("health check requested")
	
	ctx := context.Background()
	response := HealthResponse{
		Status:   "ok",
		Services: make(map[string]string),
	}

	// Check PostgreSQL
	if err := h.storage.Ping(ctx); err != nil {
		h.log.Error("storage ping failed", "error", err)
		response.Status = "degraded"
		response.Services["postgresql"] = "unavailable"
	} else {
		h.log.Debug("storage ping successful")
		response.Services["postgresql"] = "ok"
	}

	// Check Redis
	if err := h.cache.Ping(ctx); err != nil {
		h.log.Error("cache ping failed", "error", err)
		response.Status = "degraded"
		response.Services["redis"] = "unavailable"
	} else {
		h.log.Debug("cache ping successful")
		response.Services["redis"] = "ok"
	}

	statusCode := http.StatusOK
	if response.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
		h.log.Warn("health check failed", "services", response.Services)
	} else {
		h.log.Info("health check passed")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}