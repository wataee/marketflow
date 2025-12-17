package handler

import (
	"context"
	"log/slog"
	"net/http"

	"marketflow/internal/application/service"
	"marketflow/internal/domain/model"
)

type ModeHandler struct {
	modeService *service.ModeService
	switchFn    func(context.Context, model.DataMode) error
	log         *slog.Logger
}

func NewModeHandler(ms *service.ModeService, switchFn func(context.Context, model.DataMode) error, log *slog.Logger) *ModeHandler {
	return &ModeHandler{
		modeService: ms,
		switchFn:    switchFn,
		log:         log,
	}
}

func (h *ModeHandler) SwitchToTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.log.Warn("invalid method for SwitchToTest", "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.log.Info("received request to switch to test mode")
	h.switchMode(w, r, model.TestMode)
}

func (h *ModeHandler) SwitchToLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.log.Warn("invalid method for SwitchToLive", "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.log.Info("received request to switch to live mode")
	h.switchMode(w, r, model.LiveMode)
}

func (h *ModeHandler) switchMode(w http.ResponseWriter, r *http.Request, mode model.DataMode) {
	currentMode := h.modeService.GetCurrentMode()

	if currentMode == mode {
		h.log.Info("already in requested mode", "mode", mode)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","message":"already in requested mode"}`))
		return
	}

	h.log.Info("switching mode", "from", currentMode, "to", mode)

	if err := h.switchFn(r.Context(), mode); err != nil {
		h.log.Error("switch mode failed", "from", currentMode, "to", mode, "error", err)
		http.Error(w, "failed to switch mode", http.StatusInternalServerError)
		return
	}

	h.log.Info("mode switched successfully", "new_mode", mode)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","mode":"` + mode.String() + `"}`))
}
