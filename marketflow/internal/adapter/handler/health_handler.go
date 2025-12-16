package handler

import (
	"context"
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
	return &HealthHandler{storage: storage, cache: cache, log: log}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	if err := h.storage.Ping(ctx); err != nil {
		h.log.Error("storage ping failed", "error", err)
		http.Error(w, "storage unavailable", http.StatusInternalServerError)
		return
	}
	if err := h.cache.Ping(ctx); err != nil {
		h.log.Error("cache ping failed", "error", err)
		http.Error(w, "cache unavailable", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
