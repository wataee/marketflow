package handler

import (
	"encoding/json"
	"log/slog"
	"marketflow/internal/domain/port"
	"net/http"
)

type HealthHandler struct {
	storage port.StoragePort
	cache   port.CachePort
	logger  *slog.Logger
}

func NewHealthHandler(storage port.StoragePort, cache port.CachePort, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		storage: storage,
		cache:   cache,
		logger:  logger,
	}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	dbStatus := "healthy"
	redisStatus := "healthy"
	overallStatus := "healthy"

	if err := h.storage.Ping(r.Context()); err != nil {
		dbStatus = "unhealthy"
		overallStatus = "degraded"
		h.logger.Warn("database health check failed", "error", err)
	}

	if err := h.cache.Ping(r.Context()); err != nil {
		redisStatus = "unhealthy"
		overallStatus = "degraded"
		h.logger.Warn("redis health check failed", "error", err)
	}

	response := map[string]interface{}{
		"status": overallStatus,
		"checks": map[string]string{
			"database": dbStatus,
			"redis":    redisStatus,
		},
	}

	statusCode := http.StatusOK
	if overallStatus == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}