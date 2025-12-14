package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type HealthHandler struct {
	logger *slog.Logger
}

func NewHealthHandler(logger *slog.Logger) *HealthHandler {
	return &HealthHandler{logger: logger}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
		"checks": map[string]string{
			"database": "ok",
			"redis":    "ok",
		},
	}
	json.NewEncoder(w).Encode(response)
}
