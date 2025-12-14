package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"marketflow/internal/application/service"
	"marketflow/internal/domain/model"
	"net/http"
)

type ModeSwitcher func(ctx context.Context, mode model.DataMode) error

type ModeHandler struct {
	modeService *service.ModeService
	switcher    ModeSwitcher
	logger      *slog.Logger
}

func NewModeHandler(modeService *service.ModeService, switcher ModeSwitcher, logger *slog.Logger) *ModeHandler {
	return &ModeHandler{
		modeService: modeService,
		switcher:    switcher,
		logger:      logger,
	}
}

func (h *ModeHandler) SwitchToTest(w http.ResponseWriter, r *http.Request) {
	if err := h.switcher(r.Context(), model.TestMode); err != nil {
		h.logger.Error("failed to switch to test mode", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"mode":    "test",
		"message": "Switched to test mode successfully",
	})
}

func (h *ModeHandler) SwitchToLive(w http.ResponseWriter, r *http.Request) {
	if err := h.switcher(r.Context(), model.LiveMode); err != nil {
		h.logger.Error("failed to switch to live mode", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"mode":    "live",
		"message": "Switched to live mode successfully",
	})
}