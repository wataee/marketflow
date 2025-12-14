package service

import (
	"context"
	"log/slog"
	"marketflow/internal/domain/model"
	"sync"
)

type ModeService struct {
	currentMode model.DataMode
	mu          sync.RWMutex
	logger      *slog.Logger
}

func NewModeService(logger *slog.Logger) *ModeService {
	return &ModeService{
		currentMode: model.LiveMode,
		logger:      logger,
	}
}

func (s *ModeService) SwitchMode(ctx context.Context, mode model.DataMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("switching mode", "from", s.currentMode, "to", mode)
	s.currentMode = mode
	return nil
}

func (s *ModeService) GetCurrentMode() model.DataMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMode
}
