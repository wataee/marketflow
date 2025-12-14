package handler

import (
	"encoding/json"
	"log/slog"
	"marketflow/internal/application/service"
	"marketflow/internal/domain/model"
	"net/http"
)

type ModeHandler struct {
	modeService *service.ModeService
	logger      *slog.Logger
}

func NewModeHandler(modeService *service.ModeService, logger *slog.Logger) *ModeHandler {
	return &ModeHandler{
		modeService: modeService,
		logger:      logger,
	}
}

func (h *ModeHandler) SwitchToTest(w http.ResponseWriter, r *http.Request) {
	if err := h.modeService.SwitchMode(r.Context(), model.TestMode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"mode": "test", "status": "ok"})
}

func (h *ModeHandler) SwitchToLive(w http.ResponseWriter, r *http.Request) {
	if err := h.modeService.SwitchMode(r.Context(), model.LiveMode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"mode": "live", "status": "ok"})
}
