package handler

import (
	"context"
	"log/slog"
	"marketflow/internal/application/service"
	"marketflow/internal/domain/model"
	"net/http"
)

type ModeHandler struct {
	modeService *service.ModeService
	switchFn    func(context.Context, model.DataMode) error
	log         *slog.Logger
}

func NewModeHandler(ms *service.ModeService, switchFn func(context.Context, model.DataMode) error, log *slog.Logger) *ModeHandler {
	return &ModeHandler{modeService: ms, switchFn: switchFn, log: log}
}

func (h *ModeHandler) SwitchToTest(w http.ResponseWriter, r *http.Request) {
	h.switchMode(w, r, model.TestMode)
}

func (h *ModeHandler) SwitchToLive(w http.ResponseWriter, r *http.Request) {
	h.switchMode(w, r, model.LiveMode)
}

func (h *ModeHandler) switchMode(w http.ResponseWriter, r *http.Request, mode model.DataMode) {
	if err := h.switchFn(r.Context(), mode); err != nil {
		h.log.Error("switch mode failed", "error", err)
		http.Error(w, "failed to switch mode", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
